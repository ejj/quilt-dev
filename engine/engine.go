package engine

import (
	"github.com/kelda/kelda/blueprint"
	"github.com/kelda/kelda/cloud"
	"github.com/kelda/kelda/counter"
	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/join"
	"github.com/kelda/kelda/util"

	log "github.com/sirupsen/logrus"
)

var myIP = util.MyIP
var defaultDiskSize = 32

var c = counter.New("Engine")

// Run updates the database in response to changes in the blueprint table.
func Run(conn db.Conn, adminKey string) {
	for range conn.TriggerTick(30, db.BlueprintTable, db.MachineTable).C {
		conn.Txn(db.BlueprintTable, db.MachineTable).Run(
			func(view db.Database) error {
				return updateTxn(view, adminKey)
			})
	}
}

func updateTxn(view db.Database, adminKey string) error {
	c.Inc("Update")

	bp, err := view.GetBlueprint()
	if err != nil {
		return err
	}

	machineTxn(view, bp.Blueprint, adminKey)
	return nil
}

// toDBMachine converts machines specified in the blueprint into db.Machines that can
// be compared against what's already in the db.
// Specifically, it sets the role of the db.Machine, the size (which may depend
// on RAM and CPU constraints), and the provider.
// Additionally, it skips machines with invalid roles, sizes or providers.
func toDBMachine(machines []blueprint.Machine, adminKey string) []db.Machine {

	var hasMaster, hasWorker bool
	var dbMachines []db.Machine
	for _, blueprintm := range machines {
		var m db.Machine

		role, err := db.ParseRole(blueprintm.Role)
		if err != nil {
			log.WithError(err).Error("Error parsing role.")
			continue
		}
		m.Role = role

		hasMaster = hasMaster || role == db.Master
		hasWorker = hasWorker || role == db.Worker

		p, err := db.ParseProvider(blueprintm.Provider)
		if err != nil {
			log.WithError(err).Error("Error parsing provider.")
			continue
		}
		m.Provider = p
		m.Size = blueprintm.Size
		m.Preemptible = blueprintm.Preemptible

		if m.Size == "" {
			m.Size = cloud.ChooseSize(p, blueprintm.RAM, blueprintm.CPU)
			if m.Size == "" {
				log.Errorf("No valid size for %v, skipping.", m)
				continue
			}
		}

		m.DiskSize = blueprintm.DiskSize
		if m.DiskSize == 0 {
			m.DiskSize = defaultDiskSize
		}

		m.SSHKeys = blueprintm.SSHKeys
		if adminKey != "" {
			m.SSHKeys = append(m.SSHKeys, adminKey)
		}

		m.BlueprintID = blueprintm.ID
		m.Region = blueprintm.Region
		m.FloatingIP = blueprintm.FloatingIP
		dbMachines = append(dbMachines, cloud.DefaultRegion(m))
	}

	if hasMaster && !hasWorker {
		log.Warning("A Master was specified but no workers.")
		return nil
	} else if hasWorker && !hasMaster {
		log.Warning("A Worker was specified but no masters.")
		return nil
	}

	return dbMachines
}

func machineTxn(view db.Database, bp blueprint.Blueprint, adminKey string) {
	// XXX: How best to deal with machines that don't specify enough information?
	blueprintMachines := toDBMachine(bp.Machines, adminKey)

	dbMachines := view.SelectFromMachine(nil)

	scoreFun := func(left, right interface{}) int {
		blueprintMachine := left.(db.Machine)
		dbMachine := right.(db.Machine)

		switch {
		case dbMachine.BlueprintID != "" &&
			dbMachine.BlueprintID != blueprintMachine.BlueprintID:
			return -1
		case dbMachine.Provider != blueprintMachine.Provider:
			return -1
		case dbMachine.Region != blueprintMachine.Region:
			return -1
		case dbMachine.Preemptible != blueprintMachine.Preemptible:
			return -1
		case dbMachine.Size != "" && blueprintMachine.Size != dbMachine.Size:
			return -1
		case dbMachine.FloatingIP != "" &&
			dbMachine.FloatingIP != blueprintMachine.FloatingIP:
			return -1
		case dbMachine.Role != db.None && dbMachine.Role != blueprintMachine.Role:
			return -1
		case dbMachine.DiskSize != blueprintMachine.DiskSize:
			return -1
		case dbMachine.PrivateIP == "":
			return 2
		case dbMachine.PublicIP == "":
			return 1
		default:
			return 0
		}
	}

	pairs, bootList, terminateList := join.Join(blueprintMachines, dbMachines,
		scoreFun)

	for _, toTerminate := range terminateList {
		toTerminate := toTerminate.(db.Machine)
		view.Remove(toTerminate)
	}

	for _, bootSet := range bootList {
		bootSet := bootSet.(db.Machine)

		pairs = append(pairs, join.Pair{L: bootSet, R: view.InsertMachine()})
	}

	for _, pair := range pairs {
		blueprintMachine := pair.L.(db.Machine)
		dbMachine := pair.R.(db.Machine)

		dbMachine.BlueprintID = blueprintMachine.BlueprintID
		dbMachine.Role = blueprintMachine.Role
		dbMachine.Size = blueprintMachine.Size
		dbMachine.DiskSize = blueprintMachine.DiskSize
		dbMachine.Provider = blueprintMachine.Provider
		dbMachine.Region = blueprintMachine.Region
		dbMachine.SSHKeys = blueprintMachine.SSHKeys
		dbMachine.FloatingIP = blueprintMachine.FloatingIP
		dbMachine.Preemptible = blueprintMachine.Preemptible
		view.Commit(dbMachine)
	}
}

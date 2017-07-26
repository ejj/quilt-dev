package cloud

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/quilt/quilt/cloud/acl"
	"github.com/quilt/quilt/cloud/amazon"
	"github.com/quilt/quilt/cloud/cfg"
	"github.com/quilt/quilt/cloud/digitalocean"
	"github.com/quilt/quilt/cloud/foreman"
	"github.com/quilt/quilt/cloud/google"
	"github.com/quilt/quilt/cloud/machine"
	"github.com/quilt/quilt/cloud/vagrant"
	"github.com/quilt/quilt/connection"
	"github.com/quilt/quilt/counter"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/join"
	"github.com/quilt/quilt/stitch"
	"github.com/quilt/quilt/util"
)

// TODO re-order functions

type provider interface {
	List() ([]db.Machine, error)

	Boot([]db.Machine) error

	Stop([]db.Machine) error

	SetACLs([]acl.ACL) error

	UpdateFloatingIPs([]db.Machine) error
}

var c = counter.New("Cloud")

var defaultDiskSize = 32

type cloud struct {
	conn db.Conn

	namespace    string
	providerName db.ProviderName
	region       string
	provider     provider
}

var myIP = util.MyIP
var sleep = time.Sleep

// The directory from which minions will read their TLS certificates when they boot. This
// should match the location to which the daemon is installing certificates.
var minionTLSDir string

func Deploy(conn db.Conn, stitch stitch.Stitch) {
	conn.Txn(db.BlueprintTable, db.MachineTable).Run(func(view db.Database) error {
		blueprint, err := view.GetBlueprint()
		if err != nil {
			blueprint = view.InsertBlueprint()
		}

		blueprint.Stitch = stitch

		if blueprint.Namespace != stitch.Namespace {
			blueprint.Namespace = stitch.Namespace
			// TODO, explain all of this.
			for _, m := range view.SelectFromMachine(nil) {
				view.Remove(m)
			}
		}

		view.Commit(blueprint)
		return nil
	})
}

// TODO -- Track whether anything changed in the last run, and run again more
// quicly if it did (on the order of 5 seconds)

// Run continually checks 'conn' for cloud changes and recreates the cloud as
// needed.
func Run(conn db.Conn, creds connection.Credentials, minionTLSDir, adminKey_ string) {
	cfg.MinionTLSDir = minionTLSDir
	foreman.Credentials = creds
	adminKey = adminKey_

	go updateMachineStatuses(conn)
	go foreman.Run(conn, creds)

	var ns string
	stop := make(chan struct{})
	for range conn.TriggerTick(60, db.BlueprintTable, db.MachineTable).C {
		newns, _ := conn.GetBlueprintNamespace()
		if newns == ns {
			continue
		}

		log.Debugf("Namespace change from \"%s\", to \"%s\".", ns, newns)
		ns = newns

		if ns != "" {
			close(stop)
			stop = make(chan struct{})
			makeClouds(conn, ns, stop)
			foreman.Init(conn)
		}
	}
}

func makeClouds(conn db.Conn, ns string, stop chan struct{}) {
	for _, p := range db.AllProviders {
		for _, r := range validRegions(p) {
			cld, err := newCloud(conn, p, r, ns)
			if err != nil {
				log.WithFields(log.Fields{
					"error":  err,
					"region": cld.String(),
				}).Debug("failed to create cloud provider")
				continue
			}
			go cld.run(stop)
		}
	}
}

func newCloud(conn db.Conn, pName db.ProviderName, region, ns string) (cloud, error) {
	cld := cloud{
		conn:         conn,
		namespace:    ns,
		region:       region,
		providerName: pName,
	}

	var err error
	cld.provider, err = newProvider(pName, ns, region)
	if err != nil {
		return cld, fmt.Errorf("failed to connect: %s", err)
	}
	return cld, nil
}

func (cld cloud) run(stop <-chan struct{}) {
	log.Debugf("Start Cloud %s", cld)

	trigger := conn.TriggerTick(60, db.BlueprintTable, db.MachineTable)
	defer trigger.Stop()

	timeoutCount := 0
	never := make(<-chan time.Time)
	for {
		timeout := never
		if timeoutCount > 0 {
			// [1, 10] seconds
			duration := time.Duration(11-timeoutCount) * time.Second
			timeout = time.After(duration)
		}

		select {
		case <-stop:
		case <-trigger.C:
		case <-timeout:
			timeoutCount--
		}

		// In the above select, we don't want a timeout or trigger to mask a stop
		// signal.  Double check and exit just in case.
		select {
		case <-stop:
			log.Debugf("Stop Cloud %s-%s-%s", providerType, region,
				namespace)
			return
		default:
		}

		if cld.runOnce() {
			timeoutCount = 10
		}
	}
}

func (cld cloud) runOnce() bool {
	cloudMachines, err := cld.list()
	if err != nil {
		log.WithError(err).Debugf("List error %s-%s: %s",
			cld.provider, cld.region, err)
		return false
	}

	acls, machines, boot, stop, updateIPs, err := cld.transact(cloudMachines)
	if err != nil {
		log.WithError(err).Warn("Cloud transaction error.")
		return false
	}

	// TODO, what if updateMachines returns an error?
	if len(boot) > 0 {
		cld.updateMachines(boot, provider.Boot, "boot")
	}

	if len(stop) > 0 {
		cld.updateMachines(stop, provider.Stop, "stop")
	}

	if len(updateIPs) > 0 {
		cld.updateMachines(updateIPs, provider.UpdateFloatingIPs, "update IP")
	}

	if len(boot) == 0 && len(stop) == 0 && len(updateIPs) == 0 {
		cld.syncACLs(acls, machines)
		return false
	}

	return true
}

// TODO, pull this mess into it's own file
func (cld cloud) transact(cloudMachines []db.Machine) (
	acls []acl.ACL, machines, boot, stop, updateIP []db.Machine, err error) {

	mFilter := func(m db.Machine) bool {
		return cld.providerType == m.Provider && cld.region == m.Region
	}

	phase1Score := func(l, r interface{}) int {
		cm := l.(db.Machine)
		dbm := r.(db.Machine)

		if cm.CloudID == dbm.CloudID {
			return 0
		}

		// If the db machine has a cloud ID, then we must have an exact match.
		if dbm.CloudID != "" {
			return -1
		}

		if cm.Size != dbm.Size || cm.Preemptible != dbm.Preemptible ||
			(cm.DiskSize != 0 && cm.DiskSize != dbm.DiskSize) {
			return -1
		}

		return 1
	}

	phase2Score := func(l, r interface{}) int {
		dbm := l.(db.Machine)
		sm := r.(db.Machine)

		if sm.Size != dbm.Size ||
			sm.Preemptible != dbm.Preemptible ||
			(dbm.DiskSize != 0 && sm.DiskSize != dbm.DiskSize) ||
			(dbm.Role != db.None && sm.Role != dbm.Role) {
			return -1
		}

		score := 7
		if dbm.Role != db.None && dbm.Role == sm.Role {
			score -= 4
		}
		if dbm.DesiredRole != db.None && dbm.DesiredRole == sm.Role {
			score -= 2
		}
		if dbm.FloatingIP == sm.FloatingIP {
			score--
		}
		return score
	}

	txnFunc := func(view db.Database) error {
		dbcld, err := view.GetBlueprint()
		if err != nil {
			// TODO Panic
			return nil
		}
		if dbcld.Namespace != cld.namespace {
			return fmt.Errorf("abort due to namespace change expected: "+
				" \"%s\", got: \"%s", cld.namespace, dbcld.Namespace)
		}

		// TODO, test this by Manually deleting the VM when it comes up
		for _, dbm := range view.SelectFromMachine(mFilter) {
			if (dbm.Status == db.Booting || dbm.Status == db.Stopping) &&
				dbm.StatusTime.After(dbm.StatusTime.Add(5*time.Minute)) {
				// TODO log
				view.Remove(dbm)
			}
		}

		// Phase 1.
		// Update the machine table with what's currently running.
		pairs, cmis, dbmis := join.Join(db.MachineSlice(cloudMachines),
			db.MachineSlice(view.SelectFromMachine(mFilter)), phase1Score)

		for _, dbmi := range dbmis {
			dbm := dbmi.(db.Machine)
			if dbm.Status != db.Booting {
				view.Remove(dbm)
			}
		}

		for _, cmi := range cmis {
			pairs = append(pairs, join.Pair{L: cmi, R: view.InsertMachine()})
		}

		for _, pair := range pairs {
			cm := pair.L.(db.Machine)
			dbm := pair.R.(db.Machine)

			dbm.CloudID = cm.CloudID
			dbm.PublicIP = cm.PublicIP
			dbm.PrivateIP = cm.PrivateIP

			dbm.Provider = cm.Provider
			dbm.Region = cm.Region
			dbm.Size = cm.Size
			dbm.FloatingIP = cm.FloatingIP
			dbm.Preemptible = cm.Preemptible

			if cm.DiskSize != dbm.DiskSize {
				dbm.DiskSize = cm.DiskSize
			}

			view.Commit(dbm)
		}

		pairs, dbmis, smis := join.Join(view.SelectFromMachine(mFilter),
			cld.desiredMachines(dbcld.Stitch), phase2Score)

		for _, dbmi := range dbmis {
			dbm := dbmi.(db.Machine)

			// We were told to stop a VM as it was booting.  It's not
			// entirely clear how best to handle it.  For now, just remove it
			// and if it reappears later with a CloudID we can properly
			// delete it.
			if dbm.CloudID == "" {
				view.Remove(dbm)
				continue
			}

			if dbm.Status == db.Stopping {
				continue
			}

			dbm.SetStatus(db.Stopping)
			view.Commit(dbm)
			stop = append(stop, dbmi.(db.Machine))
		}

		// TODO, really think through this code .. is it wrong?
		// TODO smi is wrong
		for _, smi := range smis {
			sm := smi.(db.Machine)

			dbm := view.InsertMachine()
			dbm.SetStatus(db.Booting)

			// Set the immutable properties.  Changeable stuff is handled in
			// the next loop.
			dbm.Provider = db.Provider(sm.Provider)
			dbm.Region = sm.Region
			dbm.Size = sm.Size
			dbm.DiskSize = sm.DiskSize
			dbm.Preemptible = sm.Preemptible
			dbm.DesiredRole = sm.Role
			dbm.SSHKeys = sm.SSHKeys
			view.Commit(dbm)

			pairs = append(pairs, join.Pair{L: dbm, R: smi})
			boot = append(boot, dbm)
		}

		for _, pair := range pairs {
			dbm := pair.L.(db.Machine)
			sm := pair.R.(db.Machine)

			if dbm.CloudID != "" && dbm.FloatingIP != sm.FloatingIP {
				dbm.FloatingIP = sm.FloatingIP
				updateIP = append(updateIP, dbm)
			}

			// These thnigs change without requiring VM restarts.
			dbm.DesiredRole = sm.Role
			dbm.SSHKeys = sm.SSHKeys
			view.Commit(dbm)
		}

		machines = view.SelectFromMachine(nil)
		for acl := range cld.blueprintToACLs(bp) {
			acls = append(acls, acl)
		}

		return nil
	}

	cld.conn.Txn(db.BlueprintTable, db.MachineTable, db.ACLTable).Run(txnFunc)
	return
}

func (cld cloud) getACLs(bp db.Blueprint, machines []db.Machine) map[acl.ACL]struct{} {
	aclSet := map[acl.ACL]struct{}{}

	// Always allow traffic from the Quilt controller, so we append local.
	for _, cidr := range append(bp.AdminACL, "local") {
		acl := acl.ACL{
			CidrIP:  cidr,
			MinPort: 1,
			MaxPort: 65535,
		}
		aclSet[acl] = struct{}{}
	}

	for _, m := range machines {
		if m.PublicIP != "" {
			// XXX: Look into the minimal set of necessary ports.
			acl := acl.ACL{
				CidrIP:  m.PublicIP + "/32",
				MinPort: 1,
				MaxPort: 65535,
			}
			aclSet[acl] = struct{}{}
		}
	}

	for _, conn := range bp.Connections {
		if conn.From == stitch.PublicInternetLabel {
			acl := acl.ACL{
				CidrIP:  "0.0.0.0/0",
				MinPort: conn.MinPort,
				MaxPort: conn.MaxPort,
			}
			aclSet[acl] = struct{}{}
		}
	}

	return aclSet
}

func (cld cloud) syncACLs(unresolvedACLs []acl.ACL) {
	var acls []acl.ACL
	for _, acl := range unresolvedACLs {
		if acl.CidrIP == "local" {
			ip, err := myIP()
			if err != nil {
				log.WithError(err).Error("Failed to retrive local IP.")
				return
			}
			acl.CidrIP = ip + "/32"
		}
		acls = append(acls, acl)
	}

	c.Inc("SetACLs")
	if err := cld.provider.SetACLs(acls); err != nil {
		log.WithError(err).Warnf("Could not update ACLs in %s.", cld)
	}

	if empty {
		// For providers with no specified machines, we remove all ACLs.
		acls = nil
	}

	c.Inc("SetACLs")
	if err := cld.provider.SetACLs(acls); err != nil {
		log.WithError(err).Warnf("Could not update ACLs on %s in %s.",
			cld.provider, cld.region)
	}
}

func (cld cloud) desiredMachines(stitch stitch.Stitch) []db.Machine {
	var dbms []db.Machine
	for _, sm := range stitch.Machines {
		provider, err := db.ParseProvider(sm.Provider)
		if err != nil {
			continue // TODO log
		}

		if provider != cld.providerType {
			continue
		}

		region := defaultRegion(provider, sm.Region)
		if region != cld.region {
			continue
		}

		role, err := db.ParseRole(sm.Role)
		if err != nil {
			continue // TODO log
		}

		dbm := db.Machine{
			Region:      region,
			FloatingIP:  sm.FloatingIP,
			Role:        role,
			Provider:    provider,
			Preemptible: sm.Preemptible,
			Size:        sm.Size,
			DiskSize:    sm.DiskSize,
			SSHKeys:     sm.SSHKeys,
		}

		if dbm.Size == "" {
			dbm.Size = machine.ChooseSize(provider, sm.RAM, sm.CPU,
				stitch.MaxPrice)
			if dbm.Size == "" {
				// TODO log fmt.Errorf("no valid size for %v.", sm)
				continue
			}
		}

		if dbm.DiskSize == 0 {
			dbm.DiskSize = defaultDiskSize
		}

		if adminKey != "" {
			dbm.SSHKeys = append(dbm.SSHKeys, adminKey)
		}

		dbms = append(dbms, dbm)
	}
	return dbms
}

func (cld cloud) get() ([]db.Machine, error) {
	c.Inc("List")

	machines, err := cld.provider.List()
	if err != nil {
		return nil, fmt.Errorf("list %s: %s", cld, err)
	}

	var cloudMachines []db.Machine
	for _, m := range machines {
		m.Provider = cld.providerName
		m.Region = cld.region
		cloudMachines = append(cloudMachines, m)
	}
}

func (cld cloud) updateMachines(machines []db.Machine,
	fn func(provider, []db.Machine) error, action string) {

	location := fmt.Sprintf("%s-%s-%s", cld.providerType, cld.region, cld.namespace)
	logFields := log.Fields{
		"count":    len(machines),
		"action":   action,
		"location": location,
	}

	c.Inc(action)
	if err := fn(cld.provider, machines); err != nil {
		logFields["error"] = err
		log.WithFields(logFields).Errorf("Failed to update machines.")
	} else {
		log.WithFields(logFields).Infof("Updated machines.")
	}
}

func defaultRegion(provider db.Provider, region string) string {
	if region != "" {
		return region
	}
	switch provider {
	case db.Amazon:
		return amazon.DefaultRegion
	case db.DigitalOcean:
		return digitalocean.DefaultRegion
	case db.Google:
		return google.DefaultRegion
	case db.Vagrant:
		return ""
	default:
		panic(fmt.Sprintf("Unknown Cloud Provider: %s", provider))
	}
}

func newProviderImpl(p db.ProviderName, namespace, region string) (provider, error) {
	switch p {
	case db.Amazon:
		return amazon.New(namespace, region)
	case db.Google:
		return google.New(namespace, region)
	case db.DigitalOcean:
		return digitalocean.New(namespace, region)
	case db.Vagrant:
		return vagrant.New(namespace)
	default:
		panic("Unimplemented")
	}
}

func validRegionsImpl(p db.ProviderName) []string {
	switch p {
	case db.Amazon:
		return amazon.Regions
	case db.Google:
		return google.Zones
	case db.DigitalOcean:
		return digitalocean.Regions
	case db.Vagrant:
		return []string{""} // Vagrant has no regions
	default:
		panic("Unimplemented")
	}
}

func (cld cloud) String() string {
	return fmt.Sprintf("%s-%s-%s", cld.providerName, cld.region, cld.namespace)
}

// Stored in variables so they may be mocked out.
var newProvider = newProviderImpl
var validRegions = validRegionsImpl

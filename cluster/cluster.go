package cluster

import (
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/quilt/quilt/cluster/acl"
	"github.com/quilt/quilt/cluster/amazon"
	"github.com/quilt/quilt/cluster/cloudcfg"
	"github.com/quilt/quilt/cluster/digitalocean"
	"github.com/quilt/quilt/cluster/foreman"
	"github.com/quilt/quilt/cluster/google"
	"github.com/quilt/quilt/cluster/machine"
	"github.com/quilt/quilt/cluster/vagrant"
	"github.com/quilt/quilt/counter"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/join"
	"github.com/quilt/quilt/util"
)

type provider interface {
	List() ([]machine.Machine, error)

	Boot([]machine.Machine) error

	Stop([]machine.Machine) error

	SetACLs([]acl.ACL) error

	UpdateFloatingIPs([]machine.Machine) error
}

// Store the providers in a variable so we can change it in the tests
var allProviders = []db.Provider{db.Amazon, db.DigitalOcean, db.Google, db.Vagrant}

var c = counter.New("Cluster")

type launchLoc struct {
	provider db.Provider
	region   string
}

func (loc launchLoc) String() string {
	if loc.region == "" {
		return string(loc.provider)
	}
	return fmt.Sprintf("%s-%s", loc.provider, loc.region)
}

type cluster struct {
	namespace string
	conn      db.Conn
	providers map[launchLoc]provider
}

var myIP = util.MyIP
var sleep = time.Sleep

// action is an enum for provider actions.
type action int

const (
	boot action = iota
	stop
	updateIPs
)

// Run continually checks 'conn' for cluster changes and recreates the cluster as
// needed.
func Run(conn db.Conn) {
	var clst *cluster
	for range conn.TriggerTick(30, db.ClusterTable, db.MachineTable, db.ACLTable).C {
		c.Inc("Run")
		clst = updateCluster(conn, clst)

		// Somewhat of a crude rate-limit of once every five seconds to avoid
		// stressing out the cloud providers with too many API calls.
		sleep(5 * time.Second)
	}
}

func updateCluster(conn db.Conn, clst *cluster) *cluster {
	namespace, err := conn.GetClusterNamespace()
	if err != nil {
		return clst
	}

	if clst == nil || clst.namespace != namespace {
		clst = newCluster(conn, namespace)
		clst.runOnce()
		foreman.Init(clst.conn)
	}

	clst.runOnce()
	foreman.RunOnce(clst.conn)

	return clst
}

func newCluster(conn db.Conn, namespace string) *cluster {
	clst := &cluster{
		namespace: namespace,
		conn:      conn,
		providers: make(map[launchLoc]provider),
	}

	for _, p := range allProviders {
		for _, r := range validRegions(p) {
			if _, err := clst.getProvider(launchLoc{p, r}); err != nil {
				log.Debugf("Failed to connect to provider %s in %s: %s",
					p, r, err)
			}
		}
	}

	return clst
}

func (clst cluster) runOnce() {
	/* Each iteration of this loop does the following:
	 *
	 * - Get the current set of machines and ACLs from the cloud provider.
	 * - Get the current policy from the database.
	 * - Compute a diff.
	 * - Update the cloud provider accordingly.
	 *
	 * Updating the cloud provider may have consequences (creating machines, for
	 * example) that should be reflected in the database.  Therefore, if updates
	 * are necessary, the code loops a second time so that the database can be
	 * updated before the next runOnce() call.
	 */
	for i := 0; i < 2; i++ {
		jr, err := clst.join()
		if err != nil {
			return
		}

		if len(jr.boot) == 0 &&
			len(jr.terminate) == 0 &&
			len(jr.updateIPs) == 0 {
			// ACLs must be processed after Quilt learns about what machines
			// are in the cloud.  If we didn't, inter-machine ACLs could get
			// removed when the Quilt controller restarts, even if there are
			// running cloud machines that still need to communicate.
			clst.syncACLs(jr.acl.Admin, jr.acl.ApplicationPorts, jr.machines)
			return
		}

		clst.updateCloud(jr.boot, boot)
		clst.updateCloud(jr.terminate, stop)
		clst.updateCloud(jr.updateIPs, updateIPs)
	}
}

func (clst cluster) updateCloud(machines []joinMachine, act action) {
	if len(machines) == 0 {
		return
	}

	actionString := ""
	switch act {
	case boot:
		actionString = "boot"
	case stop:
		actionString = "stop"
	case updateIPs:
		actionString = "update floating IPs of"
	}

	log.WithField("count", len(machines)).
		Infof("Attempt to %s machines.", actionString)

	noFailures := true
	groupedMachines := groupByLoc(machines)
	for loc, providerMachines := range groupedMachines {
		providerInst, err := clst.getProvider(loc)
		if err != nil {
			noFailures = false
			log.Warnf("Provider %s is unavailable in %s: %s",
				loc.provider, loc.region, err)
			continue
		}

		switch act {
		case boot:
			c.Inc("Boot")
			err = providerInst.Boot(providerMachines)
		case stop:
			c.Inc("Stop")
			err = providerInst.Stop(providerMachines)
		case updateIPs:
			c.Inc("Update Floating IP")
			err = providerInst.UpdateFloatingIPs(providerMachines)
		}

		if err != nil {
			noFailures = false
			switch act {
			case boot:
				log.WithError(err).Warnf(
					"Unable to boot machines on %s.", loc)
			case stop:
				log.WithError(err).Warnf(
					"Unable to stop machines on %s", loc)
			case updateIPs:
				log.WithError(err).Warnf(
					"Unable to update floating IPs on %s",
					loc)
			}
		}
	}

	if noFailures {
		switch act {
		case boot:
			log.Info("Successfully booted machines.")
		case stop:
			log.Info("Successfully stopped machines")
		case updateIPs:
			log.Info("Successfully updated floating IPs")
		}
	} else {
		log.Infof("Due to failures, sleeping for 1 minute")
		sleep(60 * time.Second)
	}
}

type joinMachine struct {
	machine.Machine

	provider db.Provider
	region   string
	role     db.Role
}

type joinResult struct {
	machines []db.Machine
	acl      db.ACL

	boot      []joinMachine
	terminate []joinMachine
	updateIPs []joinMachine
}

func (clst cluster) join() (joinResult, error) {
	res := joinResult{}

	cloudMachines, err := clst.get()
	if err != nil {
		log.WithError(err).Error("Failed to list machines")
		return res, err
	}

	err = clst.conn.Txn(db.ACLTable, db.ClusterTable,
		db.MachineTable).Run(func(view db.Database) error {
		namespace, err := view.GetClusterNamespace()
		if err != nil {
			log.WithError(err).Error("Failed to get namespace")
			return err
		}

		if clst.namespace != namespace {
			err := errors.New("namespace change during a cluster run")
			log.WithError(err).Debug("Cluster run abort")
			return err
		}

		res.acl, err = view.GetACL()
		if err != nil {
			log.WithError(err).Error("Failed to get ACLs")
		}

		res.machines = view.SelectFromMachine(nil)
		cloudMachines = getMachineRoles(cloudMachines)

		dbResult := syncDB(cloudMachines, res.machines)
		res.boot = dbResult.boot
		res.terminate = dbResult.stop
		res.updateIPs = dbResult.updateIPs

		for _, pair := range dbResult.pairs {
			dbm := pair.L.(db.Machine)
			m := pair.R.(joinMachine)

			if m.role != db.None && m.role == dbm.Role {
				dbm.CloudID = m.ID
			}

			dbm.PublicIP = m.PublicIP
			dbm.PrivateIP = m.PrivateIP

			// We just booted the machine, can't possibly be connected.
			if dbm.PublicIP == "" {
				dbm.Connected = false
			}

			view.Commit(dbm)
		}
		return nil
	})
	return res, err
}

func (clst cluster) syncACLs(adminACLs []string, appACLs []db.PortRange,
	machines []db.Machine) {

	// Always allow traffic from the Quilt controller.
	ip, err := myIP()
	if err == nil {
		adminACLs = append(adminACLs, ip+"/32")
	} else {
		log.WithError(err).Error("Couldn't retrieve our IP address.")
	}

	var acls []acl.ACL
	for _, adminACL := range adminACLs {
		acls = append(acls, acl.ACL{
			CidrIP:  adminACL,
			MinPort: 1,
			MaxPort: 65535,
		})
	}
	for _, appACL := range appACLs {
		acls = append(acls, acl.ACL{
			CidrIP:  "0.0.0.0/0",
			MinPort: appACL.MinPort,
			MaxPort: appACL.MaxPort,
		})
	}

	// Providers with at least one machine.
	prvdrSet := map[launchLoc]struct{}{}
	for _, m := range machines {
		if m.PublicIP != "" {
			// XXX: Look into the minimal set of necessary ports.
			acls = append(acls, acl.ACL{
				CidrIP:  m.PublicIP + "/32",
				MinPort: 1,
				MaxPort: 65535,
			})
		}
		prvdrSet[launchLoc{m.Provider, m.Region}] = struct{}{}
	}

	for loc, prvdr := range clst.providers {
		// For providers with no specified machines, we remove all ACLs.
		// Otherwise we set acls to what's specified.
		var setACLs []acl.ACL
		if _, ok := prvdrSet[loc]; ok {
			setACLs = acls
		}

		c.Inc("SetACLs")
		if err := prvdr.SetACLs(setACLs); err != nil {
			log.WithError(err).Warnf("Could not update ACLs on %s in %s.",
				loc.provider, loc.region)
		}
	}
}

type syncDBResult struct {
	pairs     []join.Pair
	boot      []joinMachine
	stop      []joinMachine
	updateIPs []joinMachine
}

func syncDB(cms []joinMachine, dbms []db.Machine) syncDBResult {
	ret := syncDBResult{}

	pair1, dbmis, cmis := join.Join(dbms, cms, func(l, r interface{}) int {
		dbm := l.(db.Machine)
		m := r.(joinMachine)

		if dbm.CloudID == m.ID && dbm.Provider == m.provider &&
			dbm.Preemptible == m.Preemptible &&
			dbm.Region == m.region && dbm.Size == m.Size &&
			(m.DiskSize == 0 || dbm.DiskSize == m.DiskSize) &&
			(m.role == db.None || dbm.Role == m.role) {
			return 0
		}

		return -1
	})

	pair2, dbmis, cmis := join.Join(dbmis, cmis, func(l, r interface{}) int {
		dbm := l.(db.Machine)
		m := r.(joinMachine)

		if dbm.Provider != m.provider ||
			dbm.Region != m.region ||
			dbm.Size != m.Size ||
			dbm.Preemptible != m.Preemptible ||
			(m.DiskSize != 0 && dbm.DiskSize != m.DiskSize) ||
			(m.role != db.None && dbm.Role != m.role) {
			return -1
		}

		score := 10
		if dbm.Role != db.None && m.role != db.None && dbm.Role == m.role {
			score -= 4
		}
		if dbm.PublicIP == m.PublicIP && dbm.PrivateIP == m.PrivateIP {
			score -= 2
		}
		if dbm.FloatingIP == m.FloatingIP {
			score--
		}
		return score
	})

	for _, cm := range cmis {
		ret.stop = append(ret.stop, cm.(joinMachine))
	}

	for _, dbm := range dbmis {
		m := dbm.(db.Machine)
		ret.boot = append(ret.boot, joinMachine{
			Machine: machine.Machine{
				Size:        m.Size,
				DiskSize:    m.DiskSize,
				Preemptible: m.Preemptible,
				CloudCfgOpts: cloudcfg.Options{
					SSHKeys: m.SSHKeys,
					MinionOpts: cloudcfg.MinionOptions{
						Role: m.Role,
					},
				},
			},
			provider: m.Provider,
			region:   m.Region,
		})
	}

	for _, pair := range append(pair1, pair2...) {
		dbm := pair.L.(db.Machine)
		m := pair.R.(joinMachine)

		if dbm.CloudID == m.ID && dbm.FloatingIP != m.FloatingIP {
			m.FloatingIP = dbm.FloatingIP
			ret.updateIPs = append(ret.updateIPs, m)
		}

		ret.pairs = append(ret.pairs, pair)
	}

	return ret
}

type listResponse struct {
	loc      launchLoc
	machines []machine.Machine
	err      error
}

func (clst cluster) get() ([]joinMachine, error) {
	var wg sync.WaitGroup
	cloudMachinesChan := make(chan listResponse, len(clst.providers))
	for loc, p := range clst.providers {
		wg.Add(1)
		go func(loc launchLoc, p provider) {
			defer wg.Done()
			c.Inc("List")
			machines, err := p.List()
			cloudMachinesChan <- listResponse{loc, machines, err}
		}(loc, p)
	}
	wg.Wait()
	close(cloudMachinesChan)

	var cloudMachines []joinMachine
	for res := range cloudMachinesChan {
		if res.err != nil {
			return nil, fmt.Errorf("list %s: %s", res.loc, res.err)
		}
		for _, m := range res.machines {
			cloudMachines = append(cloudMachines, joinMachine{
				Machine:  m,
				provider: res.loc.provider,
				region:   res.loc.region,
			})
		}
	}
	return cloudMachines, nil
}

func (clst cluster) getProvider(loc launchLoc) (provider, error) {
	p, ok := clst.providers[loc]
	if ok {
		return p, nil
	}

	p, err := newProvider(loc.provider, clst.namespace, loc.region)
	if err == nil {
		clst.providers[loc] = p
	}
	return p, err
}

func getMachineRoles(machines []joinMachine) (withRoles []joinMachine) {
	for _, m := range machines {
		m.role = getMachineRole(m.PublicIP)
		withRoles = append(withRoles, m)
	}
	return withRoles
}

func groupByLoc(machines []joinMachine) map[launchLoc][]machine.Machine {
	machineMap := map[launchLoc][]machine.Machine{}
	for _, m := range machines {
		loc := launchLoc{m.provider, m.region}
		machineMap[loc] = append(machineMap[loc], m.Machine)
	}

	return machineMap
}

func newProviderImpl(p db.Provider, namespace, region string) (provider, error) {
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

func validRegionsImpl(p db.Provider) []string {
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

// Stored in variables so they may be mocked out
var newProvider = newProviderImpl
var validRegions = validRegionsImpl
var getMachineRole = foreman.GetMachineRole

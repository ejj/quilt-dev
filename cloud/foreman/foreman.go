package foreman

import (
	"context"
	"errors"
	"reflect"
	"time"

	"google.golang.org/grpc"

	"github.com/quilt/quilt/connection"
	"github.com/quilt/quilt/counter"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/minion/pb"

	log "github.com/Sirupsen/logrus"
)

// Credentials that the foreman should use to connect to its minions.
var Credentials connection.Credentials

// TODO, as an incremental thing, pull this into one big thread, then later we can do a
// thread per foreman.

type client interface {
	setMinion(pb.MinionConfig) error
	getMinion() (pb.MinionConfig, error)
	Close()
}

type clientImpl struct {
	pb.MinionClient
	cc *grpc.ClientConn
}

type update struct {
	ip     string
	role   db.Role
	status string
}

type foreman struct {
	client client
	conn   db.Conn
	ip     string
	status string

	exitChan   chan<- string
	updateChan chan<- update
}

var c = counter.New("Foreman")

var creds connection.Credentials

func Run(conn db.Conn, _creds connection.Credentials) {
	creds = _creds

	updateChan := make(chan update, 32)
	go updateRoutine(conn, updateChan)

	threads := map[string]struct{}{}
	triggerChan := conn.Trigger(db.MachineTable)
	exitChan := make(chan string, 32) // TODO comment
	for {
		select {
		case exited := <-exitChan:
			delete(threads, exited)
		case <-triggerChan.C:
		}

		for {
			// Drain exitChan.
			select {
			case exited := <-exitChan:
				delete(threads, exited)
				continue
			default:
			}
			break
		}

		dbms := conn.SelectFromMachine(func(m db.Machine) bool {
			return m.PublicIP != "" && m.PrivateIP != "" &&
				m.Status != db.Stopping
		})

		for _, dbm := range dbms {
			if _, ok := threads[dbm.PublicIP]; !ok {
				threads[dbm.PublicIP] = struct{}{}
				fm := foreman{
					conn:       conn,
					ip:         dbm.PublicIP,
					updateChan: updateChan,
					exitChan:   exitChan,
				}
				go fm.run()
			}
		}
	}
}

// TODO, comment explaining why we have this function insteadof the foreman doing the
// update themselves.
func updateRoutine(conn db.Conn, machineChan <-chan update) {
	for m := range machineChan {
		updateMap := map[string]update{}
		updateMap[m.ip] = m

		// Give other threads a chance to fill up machineChan.
		time.Sleep(250 * time.Millisecond)

		for {
			select {
			case m := <-machineChan:
				updateMap[m.ip] = m
				continue
			default:
			}
			break
		}

		conn.Txn(db.MachineTable).Run(func(view db.Database) error {
			dbms := view.SelectFromMachine(func(m db.Machine) bool {
				return m.PublicIP != "" && m.Status != db.Stopping
			})

			for _, dbm := range dbms {
				update, ok := updateMap[dbm.PublicIP]
				if ok {
					if update.status != "" {
						dbm.Status = update.status
					}
					if update.role != db.None {
						dbm.Role = update.role
					}
					view.Commit(dbm)
				}
			}
			return nil
		})
	}
}

func (f foreman) run() {
	defer func() {
		log.WithField("ip", f.ip).Debug("Foreman Exit")
		f.exitChan <- f.ip
	}()
	log.WithField("ip", f.ip).Debug("Foreman Start")

	trigger := f.conn.TriggerTick(60, db.BlueprintTable, db.MachineTable)
	fast := time.NewTicker(5 * time.Second)

	defer trigger.Stop()
	defer fast.Stop()

	for {
		select {
		case <-trigger.C:
		case <-fast.C:
			if f.status == db.Connected {
				continue
			}
		}

		if err := f.runOnce(); err != nil {
			return
		}
	}
}

// TODO test restarting the daemon (and the resulting role changes)

// TODO counter the hell out of all this stuff
func (f *foreman) runOnce() error {
	var dbms []db.Machine
	var bp db.Blueprint

	f.conn.Txn(db.BlueprintTable, db.MachineTable).Run(func(view db.Database) error {
		dbms = view.SelectFromMachine(func(m db.Machine) bool {
			return m.Status != db.Stopping
		})
		bp, _ = view.GetBlueprint() // TODO assert
		return nil
	})

	missing := true
	var dbm db.Machine
	var etcdIPs []string
	for _, m := range dbms {
		if m.PublicIP == f.ip {
			dbm = m
			missing = false
		}

		if m.Role == db.Master && m.PrivateIP != "" {
			etcdIPs = append(etcdIPs, m.PrivateIP)

		}
	}
	if missing {
		return errors.New("missing machine")
	}

	f.status = dbm.Status

	if f.client == nil {
		f.setStatus(db.Connecting)

		var err error
		f.client, err = newClient(f.ip)
		if err != nil {
			// TODO
			// log.WithError(err).Debugf("Failed to connect to %s", f.ip)
			return nil
		}
	}

	cfg, err := f.client.getMinion()
	if err != nil {
		log.WithError(err).Debugf("Failed to get minion config from %s", f.ip)
		f.setStatus(db.Connecting)
		f.client = nil
		return nil
	}

	f.setStatus(db.Connected)

	role := db.PBToRole(cfg.Role)
	if role != db.None && role != dbm.Role {
		f.setRole(role)
	}

	newConfig := pb.MinionConfig{
		FloatingIP:     dbm.FloatingIP,
		PrivateIP:      dbm.PrivateIP,
		Blueprint:      bp.Stitch.String(),
		Provider:       string(dbm.Provider),
		Size:           dbm.Size,
		Region:         dbm.Region,
		EtcdMembers:    etcdIPs,
		AuthorizedKeys: dbm.SSHKeys,
	}

	if reflect.DeepEqual(newConfig, cfg) {
		return nil
	}

	if err := f.client.setMinion(newConfig); err != nil {
		log.WithError(err).Debugf("Failed to set minion config on %s.", f.ip)
		f.setStatus(db.Connecting)
		f.client = nil
		return nil
	}

	return nil
}

// Note that setStatus and setRole fail silently if the machine we're looking for is
// missing.  The caller will close itself on its own on the next run through
func (f foreman) setStatus(status string) {
	if f.status != status {
		f.status = status
		f.updateChan <- update{ip: f.ip, status: status}
	}
}

func (f foreman) setRole(role db.Role) {
	f.updateChan <- update{ip: f.ip, role: role}
}

func newClientImpl(ip string) (client, error) {
	c.Inc("New Minion Client")
	cc, err := connection.Client("tcp", ip+":9999", Credentials.ClientOpts())
	if err != nil {
		c.Inc("New Minion Client Error")
		return nil, err
	}

	return clientImpl{pb.NewMinionClient(cc), cc}, nil
}

// Storing in a variable allows us to mock it out for unit tests
var newClient = newClientImpl

func (cl clientImpl) getMinion() (pb.MinionConfig, error) {
	c.Inc("Get Minion")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cfg, err := cl.GetMinionConfig(ctx, &pb.Request{})
	if err != nil {
		c.Inc("Get Minion Error")
		return pb.MinionConfig{}, err
	}

	return *cfg, nil
}

func (cl clientImpl) setMinion(cfg pb.MinionConfig) error {
	c.Inc("Set Minion")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	_, err := cl.SetMinionConfig(ctx, &cfg)
	if err != nil {
		c.Inc("Set Minion Error")
	}
	return err
}

func (cl clientImpl) Close() {
	c.Inc("Close Client")
	cl.cc.Close()
}

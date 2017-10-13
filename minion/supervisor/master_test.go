package supervisor

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/minion/supervisor/images"
)

func TestNone(t *testing.T) {
	ctx := initTest(db.Master)

	if len(ctx.fd.running()) > 0 {
		t.Errorf("fd.running = %s; want <empty>", spew.Sdump(ctx.fd.running()))
	}

	if len(ctx.execs) > 0 {
		t.Errorf("exec = %s; want <empty>", spew.Sdump(ctx.execs))
	}

	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.MinionSelf()
		e := view.SelectFromEtcd(nil)[0]
		m.PrivateIP = "1.2.3.4"
		e.Leader = false
		e.LeaderIP = "5.6.7.8"
		view.Commit(m)
		view.Commit(e)
		return nil
	})
	ctx.run()

	if len(ctx.fd.running()) > 0 {
		t.Errorf("fd.running = %s; want <none>", spew.Sdump(ctx.fd.running()))
	}

	if len(ctx.execs) > 0 {
		t.Errorf("exec = %s; want <empty>", spew.Sdump(ctx.execs))
	}
}

func TestMaster(t *testing.T) {
	ctx := initTest(db.Master)
	ip := "1.2.3.4"
	etcdIPs := []string{""}
	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.MinionSelf()
		e := view.SelectFromEtcd(nil)[0]
		m.Role = db.Master
		m.PrivateIP = ip
		e.EtcdIPs = etcdIPs
		view.Commit(m)
		view.Commit(e)
		return nil
	})
	ctx.run()

	exp := map[string][]string{
		images.Etcd:     etcdArgsMaster(ip, etcdIPs),
		images.Ovsdb:    {"ovsdb-server"},
		images.Registry: nil,
	}
	if !reflect.DeepEqual(ctx.fd.running(), exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(ctx.fd.running()),
			spew.Sdump(exp))
	}

	if len(ctx.execs) > 0 {
		t.Errorf("exec = %s; want <empty>", spew.Sdump(ctx.execs))
	}

	/* Change IP, etcd IPs, and become the leader. */
	ip = "8.8.8.8"
	etcdIPs = []string{"8.8.8.8"}
	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.MinionSelf()
		e := view.SelectFromEtcd(nil)[0]
		m.Role = db.Master
		m.PrivateIP = ip
		e.EtcdIPs = etcdIPs
		e.Leader = true
		view.Commit(m)
		view.Commit(e)
		return nil
	})
	ctx.run()

	exp = map[string][]string{
		images.Etcd:      etcdArgsMaster(ip, etcdIPs),
		images.Ovsdb:     {"ovsdb-server"},
		images.Ovnnorthd: {"ovn-northd"},
		images.Registry:  nil,
	}
	if !reflect.DeepEqual(ctx.fd.running(), exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(ctx.fd.running()),
			spew.Sdump(exp))
	}
	if len(ctx.execs) > 0 {
		t.Errorf("exec = %s; want <empty>", spew.Sdump(ctx.execs))
	}

	/* Lose leadership. */
	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		e := view.SelectFromEtcd(nil)[0]
		e.Leader = false
		view.Commit(e)
		return nil
	})
	ctx.run()

	exp = map[string][]string{
		images.Etcd:     etcdArgsMaster(ip, etcdIPs),
		images.Ovsdb:    {"ovsdb-server"},
		images.Registry: nil,
	}
	if !reflect.DeepEqual(ctx.fd.running(), exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(ctx.fd.running()),
			spew.Sdump(exp))
	}
	if len(ctx.execs) > 0 {
		t.Errorf("exec = %s; want <empty>", spew.Sdump(ctx.execs))
	}
}

func TestEtcdAdd(t *testing.T) {
	ctx := initTest(db.Master)
	ip := "1.2.3.4"
	etcdIPs := []string{ip, "5.6.7.8"}
	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.MinionSelf()
		e := view.SelectFromEtcd(nil)[0]
		m.Role = db.Master
		m.PrivateIP = ip
		e.EtcdIPs = etcdIPs
		view.Commit(m)
		view.Commit(e)
		return nil
	})
	ctx.run()

	exp := map[string][]string{
		images.Etcd:     etcdArgsMaster(ip, etcdIPs),
		images.Ovsdb:    {"ovsdb-server"},
		images.Registry: nil,
	}
	if !reflect.DeepEqual(ctx.fd.running(), exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(ctx.fd.running()),
			spew.Sdump(exp))
	}

	// Add a new master
	etcdIPs = append(etcdIPs, "9.10.11.12")
	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.MinionSelf()
		e := view.SelectFromEtcd(nil)[0]
		m.Role = db.Master
		e.EtcdIPs = etcdIPs
		view.Commit(m)
		view.Commit(e)
		return nil
	})
	ctx.run()

	exp = map[string][]string{
		images.Etcd:     etcdArgsMaster(ip, etcdIPs),
		images.Ovsdb:    {"ovsdb-server"},
		images.Registry: nil,
	}
	if !reflect.DeepEqual(ctx.fd.running(), exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(ctx.fd.running()),
			spew.Sdump(exp))
	}
}

func TestEtcdRemove(t *testing.T) {
	ctx := initTest(db.Master)
	ip := "1.2.3.4"
	etcdIPs := []string{ip, "5.6.7.8"}
	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.MinionSelf()
		e := view.SelectFromEtcd(nil)[0]
		m.Role = db.Master
		m.PrivateIP = ip
		e.EtcdIPs = etcdIPs
		view.Commit(m)
		view.Commit(e)
		return nil
	})
	ctx.run()

	exp := map[string][]string{
		images.Etcd:     etcdArgsMaster(ip, etcdIPs),
		images.Ovsdb:    {"ovsdb-server"},
		images.Registry: nil,
	}
	if !reflect.DeepEqual(ctx.fd.running(), exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(ctx.fd.running()),
			spew.Sdump(exp))
	}

	// Remove a master
	etcdIPs = etcdIPs[1:]
	ctx.conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.MinionSelf()
		e := view.SelectFromEtcd(nil)[0]
		m.Role = db.Master
		e.EtcdIPs = etcdIPs
		view.Commit(m)
		view.Commit(e)
		return nil
	})
	ctx.run()

	exp = map[string][]string{
		images.Etcd:     etcdArgsMaster(ip, etcdIPs),
		images.Ovsdb:    {"ovsdb-server"},
		images.Registry: nil,
	}
	if !reflect.DeepEqual(ctx.fd.running(), exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(ctx.fd.running()),
			spew.Sdump(exp))
	}
}

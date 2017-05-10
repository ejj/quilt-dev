package etcd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/join"

	log "github.com/Sirupsen/logrus"
)

const hostnamePath = "/hostnames"

func runHostname(conn db.Conn, store Store) {
	etcdWatch := store.Watch(hostnamePath, 1*time.Second)
	trigg := conn.TriggerTick(60, db.HostnameTable)
	for range joinNotifiers(trigg.C, etcdWatch) {
		if err := runHostnameOnce(conn, store); err != nil {
			log.WithError(err).Warn("Failed to sync hostnames with Etcd")
		}
	}
}

func runHostnameOnce(conn db.Conn, store Store) error {
	etcdStr, err := readEtcdNode(store, hostnamePath)
	if err != nil {
		return fmt.Errorf("etcd read error: %s", err)
	}

	if conn.EtcdLeader() {
		c.Inc("Run Hostname Leader")
		hostnames := db.HostnameSlice(conn.SelectFromHostname(nil))
		err := writeEtcdSlice(store, hostnamePath, etcdStr, hostnames)
		if err != nil {
			return fmt.Errorf("etcd write error: %s", err)
		}
	} else {
		c.Inc("Run Hostname Worker")
		var etcdHostnames []db.Hostname
		json.Unmarshal([]byte(etcdStr), &etcdHostnames)
		conn.Txn(db.HostnameTable).Run(func(view db.Database) error {
			joinHostnames(view, etcdHostnames)
			return nil
		})
	}

	return nil
}

func joinHostnames(view db.Database, etcdHostnames []db.Hostname) {
	key := func(iface interface{}) interface{} {
		h := iface.(db.Hostname)
		h.ID = 0
		return h
	}
	_, dbIfaces, etcdIfaces := join.HashJoin(
		db.HostnameSlice(view.SelectFromHostname(nil)),
		db.HostnameSlice(etcdHostnames), key, key)

	for _, iface := range dbIfaces {
		view.Remove(iface.(db.Hostname))
	}

	for _, iface := range etcdIfaces {
		etcdHostname := iface.(db.Hostname)
		dbHostname := view.InsertHostname()
		etcdHostname.ID = dbHostname.ID
		view.Commit(etcdHostname)
	}
}

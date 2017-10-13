package etcd

import (
	"time"

	"github.com/coreos/etcd/client"
	"github.com/kelda/kelda/counter"
	"github.com/kelda/kelda/db"

	log "github.com/sirupsen/logrus"
)

const electionTTL = 30
const leaderKey = "/leader"

var electorC = counter.New("Etcd Elector")

// Run blocks implementing leader election.
func runElection(conn db.Conn, store Store) {
	go watchLeader(conn, store)
	campaign(conn, store)
}

func watchLeader(conn db.Conn, store Store) {
	tickRate := electionTTL
	if tickRate > 30 {
		tickRate = 30
	}

	watch := store.Watch(leaderKey, 1*time.Second)
	trigg := conn.TriggerTick(tickRate, db.EtcdTable)
	for {
		electorC.Inc("Update Leader")
		leader, _ := store.Get(leaderKey)
		conn.Txn(db.EtcdTable).Run(func(view db.Database) error {
			etcdRows := view.SelectFromEtcd(nil)
			if len(etcdRows) == 1 {
				etcdRows[0].LeaderIP = leader
				view.Commit(etcdRows[0])
			}
			return nil
		})

		select {
		case <-watch:
		case <-trigg.C:
		}
	}
}

func campaign(conn db.Conn, store Store) {
	watch := store.Watch(leaderKey, 1*time.Second)
	trigg := conn.TriggerTick(electionTTL/2, db.EtcdTable)

	for {
		select {
		case <-watch:
		case <-trigg.C:
		}

		c.Inc("Campaign")
		etcdRows := conn.SelectFromEtcd(nil)

		minion := conn.MinionSelf()
		master := minion.Role == db.Master && len(etcdRows) == 1

		if !master {
			continue
		}

		IP := minion.PrivateIP
		if IP == "" {
			continue
		}

		ttl := electionTTL * time.Second

		var err error
		if etcdRows[0].Leader {
			err = store.Refresh(leaderKey, IP, ttl)
		} else {
			err = store.Create(leaderKey, IP, ttl)
		}

		if err == nil {
			c.Inc("Campaign Won")
			commitLeader(conn, true, IP)
		} else {
			c.Inc("Campaign Lost")
			clientErr, ok := err.(client.Error)
			if !ok || clientErr.Code != client.ErrorCodeNodeExist {
				log.WithError(err).Warn("Error setting leader key")
				commitLeader(conn, false, "")

				// Give things a chance to settle down.
				time.Sleep(electionTTL * time.Second)
			} else {
				commitLeader(conn, false)
			}
		}
	}
}

func commitLeader(conn db.Conn, leader bool, ip ...string) {
	if len(ip) > 1 {
		panic("Not Reached")
	}

	conn.Txn(db.EtcdTable).Run(func(view db.Database) error {
		etcdRows := view.SelectFromEtcd(nil)
		if len(etcdRows) == 1 {
			etcdRows[0].Leader = leader

			if len(ip) == 1 {
				etcdRows[0].LeaderIP = ip[0]
			}

			view.Commit(etcdRows[0])
		}
		return nil
	})
}

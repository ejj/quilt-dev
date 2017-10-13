package etcd

import (
	"time"

	"github.com/coreos/etcd/client"
	"github.com/kelda/kelda/db"

	log "github.com/sirupsen/logrus"
)

// Run synchronizes state in `conn` with the Etcd cluster.
func Run(conn db.Conn) {
	store := NewStore()
	makeEtcdDir(minionPath, store, 0)

	go runElection(conn, store)
	go runConnection(conn, store)
	go runContainer(conn, store)
	go runHostname(conn, store)
	runMinionSync(conn, store)
}

func makeEtcdDir(dir string, store Store, ttl time.Duration) {
	for {
		err := createEtcdDir(dir, store, ttl)
		if err == nil {
			break
		}

		log.WithError(err).Debug("Failed to create etcd dir")
		time.Sleep(5 * time.Second)
	}
}

func createEtcdDir(dir string, store Store, ttl time.Duration) error {
	err := store.Mkdir(dir, ttl)
	if err == nil {
		return nil
	}

	// If the directory already exists, no need to create it
	etcdErr, ok := err.(client.Error)
	if ok && etcdErr.Code == client.ErrorCodeNodeExist {
		return store.RefreshDir(dir, ttl)
	}

	return err
}

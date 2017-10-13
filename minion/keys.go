package minion

import (
	"os"
	"time"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/util"

	log "github.com/sirupsen/logrus"
)

const authorizedKeysFile = "/home/quilt/.ssh/authorized_keys"

func syncAuthorizedKeys(conn db.Conn) {
	// XXX: If we immediately started syncing the SSH keys, there would be a
	// brief period where no keys are installed. This is because
	// Minion.AuthorizedKeys is not populated until the foreman connects;
	// therefore, before the foreman sends the MinionConfig over, we would
	// overwrite the keys installed by the cloud config. To alleviate this, we
	// do not touch the keys until the foreman has sent us some keys to
	// install.
	waitForConfig(conn)

	for range conn.TriggerTick(30, db.MinionTable).C {
		if err := runOnce(conn); err != nil {
			log.WithError(err).Error("Failed to sync keys")
		}
	}
}

func runOnce(conn db.Conn) error {
	if _, err := util.AppFs.Stat(authorizedKeysFile); os.IsNotExist(err) {
		util.AppFs.Create(authorizedKeysFile)
	}
	currKeys, err := util.ReadFile(authorizedKeysFile)
	if err != nil {
		return err
	}

	m := conn.MinionSelf()

	if m.AuthorizedKeys == currKeys {
		return nil
	}

	c.Inc("Change SSH Keys")
	return util.WriteFile(authorizedKeysFile, []byte(m.AuthorizedKeys), 0644)
}

func waitForConfig(conn db.Conn) {
	for {
		if conn.MinionSelf().AuthorizedKeys != "" {
			return
		}
		time.Sleep(1 * time.Second)
	}
}

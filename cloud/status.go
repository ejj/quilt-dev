package cloud

import (
	"github.com/kelda/kelda/cloud/foreman"
	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/util"
)

func updateMachineStatuses(conn db.Conn) {
	dbTrig := conn.TriggerTick(30, db.MachineTable).C
	for range util.JoinNotifiers(dbTrig, foreman.ConnectionTrigger) {
		updateMachineStatusesOnce(conn)
	}
}

func updateMachineStatusesOnce(conn db.Conn) {
	conn.Txn(db.MachineTable).Run(func(view db.Database) error {
		for _, dbm := range view.SelectFromMachine(nil) {
			// Don't touch machines that are still booting.  Note that we
			// can't check `dbm.Status == db.Booting`, because this is the
			// code responsible for changing it from db.Booting to
			// db.Connecting and it must run to achieve that.
			if dbm.PublicIP == "" {
				continue
			}

			if newStatus, ok := status(dbm); ok && newStatus != dbm.Status {
				dbm.Status = newStatus
				view.Commit(dbm)
			}
		}
		return nil
	})
}

// status returns a status string for the given machine. If no string could be
// determined, the second return value is false.
func status(m db.Machine) (string, bool) {
	// "Connected" takes priority over other statuses.
	connected := m.PublicIP != "" && isConnected(m.PublicIP)
	if connected {
		return db.Connected, true
	}

	// If we had previously connected, and we are not currently connected, show
	// that we are attempting to reconnect.
	if m.Status == db.Connected || m.Status == db.Reconnecting {
		return db.Reconnecting, true
	}

	// If we've never successfully connected, but have booted enough to have a
	// public IP, show that we are attempting to connect.
	if m.PublicIP != "" {
		return db.Connecting, true
	}

	return "", false
}

var isConnected = foreman.IsConnected

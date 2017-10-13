package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kelda/kelda/db"
)

func TestUpdateMachineStatuses(t *testing.T) {
	isConnected = func(host string) bool {
		switch host {
		case "connect-fail":
			return false
		case "connect-succeed":
			return true
		default:
			panic("unrecognized host")
		}
	}

	conn := db.New()
	conn.Txn(db.MachineTable).Run(func(view db.Database) error {
		// An unbooted machine.
		m := view.InsertMachine()
		m.BlueprintID = "1"
		view.Commit(m)

		// A booting machine.
		m = view.InsertMachine()
		m.BlueprintID = "2"
		m.Status = db.Booting
		view.Commit(m)

		// A booted machine trying to connect.
		m = view.InsertMachine()
		m.BlueprintID = "3"
		m.Status = ""
		m.PublicIP = "connect-fail"
		view.Commit(m)

		// Another booted machine trying to connect.
		m = view.InsertMachine()
		m.BlueprintID = "4"
		m.Status = db.Connecting
		m.PublicIP = "connect-fail"
		view.Commit(m)

		// A connecting machine that succeeds.
		m = view.InsertMachine()
		m.BlueprintID = "5"
		m.Status = db.Connecting
		m.PublicIP = "connect-succeed"
		view.Commit(m)

		// A connected machine that disconnects.
		m = view.InsertMachine()
		m.BlueprintID = "6"
		m.Status = db.Connected
		m.PublicIP = "connect-fail"
		view.Commit(m)

		// A reconnecting machine that fails to reconnect.
		m = view.InsertMachine()
		m.BlueprintID = "7"
		m.Status = db.Reconnecting
		m.PublicIP = "connect-fail"
		view.Commit(m)

		return nil
	})

	updateMachineStatusesOnce(conn)

	actual := conn.SelectFromMachine(nil)
	for i := range actual {
		actual[i].ID = 0
		actual[i].PublicIP = ""
	}
	assert.Len(t, actual, 7)
	assert.Contains(t, actual, db.Machine{BlueprintID: "1"})
	assert.Contains(t, actual, db.Machine{BlueprintID: "2", Status: db.Booting})
	assert.Contains(t, actual, db.Machine{BlueprintID: "3", Status: db.Connecting})
	assert.Contains(t, actual, db.Machine{BlueprintID: "4", Status: db.Connecting})
	assert.Contains(t, actual, db.Machine{BlueprintID: "5", Status: db.Connected})
	assert.Contains(t, actual, db.Machine{BlueprintID: "6", Status: db.Reconnecting})
	assert.Contains(t, actual, db.Machine{BlueprintID: "7", Status: db.Reconnecting})
}

package etcd

import (
	"testing"

	"github.com/kelda/kelda/db"
	"github.com/stretchr/testify/assert"
)

func TestRunContainerOnce(t *testing.T) {
	t.Parallel()

	store := newTestMock()
	conn := db.New()

	err := runContainerOnce(conn, store)
	assert.Error(t, err)

	err = store.Set(containerPath, "", 0)
	assert.NoError(t, err)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		self := view.InsertMinion()
		self.Self = true
		self.Role = db.Master
		view.Commit(self)

		etcd := view.InsertEtcd()
		etcd.Leader = true
		view.Commit(etcd)

		dbc := view.InsertContainer()
		dbc.Hostname = "host"
		dbc.IP = "10.0.0.2"
		dbc.Minion = "1.2.3.4"
		dbc.BlueprintID = "12"
		dbc.Image = "ubuntu"
		dbc.Command = []string{"1", "2", "3"}
		dbc.Env = map[string]string{"red": "pill", "blue": "pill"}
		dbc.FilepathToContent = map[string]string{"foo": "bar"}
		view.Commit(dbc)
		return nil
	})

	err = runContainerOnce(conn, store)
	assert.NoError(t, err)

	str, err := store.Get(containerPath)
	assert.NoError(t, err)

	expStr := `[
    {
        "IP": "10.0.0.2",
        "Minion": "1.2.3.4",
        "BlueprintID": "12",
        "Command": [
            "1",
            "2",
            "3"
        ],
        "Env": {
            "blue": "pill",
            "red": "pill"
        },
        "FilepathToContent": {
            "foo": "bar"
        },
        "Hostname": "host",
        "Created": "0001-01-01T00:00:00Z",
        "Image": "ubuntu"
    }
]`
	assert.Equal(t, expStr, str)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		etcd := view.SelectFromEtcd(nil)[0]
		etcd.Leader = false
		view.Commit(etcd)

		dbc := view.SelectFromContainer(nil)[0]
		dbc.Env = map[string]string{"red": "fish", "blue": "fish"}
		dbc.FilepathToContent = map[string]string{"bar": "baz"}
		view.Commit(dbc)
		return nil
	})

	err = runContainerOnce(conn, store)
	assert.NoError(t, err)

	expDBC := db.Container{
		IP:                "10.0.0.2",
		BlueprintID:       "12",
		Minion:            "1.2.3.4",
		Image:             "ubuntu",
		Command:           []string{"1", "2", "3"},
		Env:               map[string]string{"red": "pill", "blue": "pill"},
		FilepathToContent: map[string]string{"foo": "bar"},
		Hostname:          "host",
	}
	dbcs := conn.SelectFromContainer(nil)
	assert.Len(t, dbcs, 1)
	dbcs[0].ID = 0
	assert.Equal(t, expDBC, dbcs[0])

	err = runContainerOnce(conn, store)
	assert.NoError(t, err)

	dbcs = conn.SelectFromContainer(nil)
	assert.Len(t, dbcs, 1)
	dbcs[0].ID = 0
	assert.Equal(t, expDBC, dbcs[0])

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		self := view.MinionSelf()
		self.Role = db.Worker
		self.PrivateIP = "1.2.3.4"
		view.Commit(self)
		return nil
	})

	err = runContainerOnce(conn, store)
	assert.NoError(t, err)

	dbcs = conn.SelectFromContainer(nil)
	assert.Len(t, dbcs, 1)
	dbcs[0].ID = 0
	assert.Equal(t, expDBC, dbcs[0])

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		self := view.MinionSelf()
		self.PrivateIP = "1.2.3.5"
		view.Commit(self)
		return nil
	})

	err = runContainerOnce(conn, store)
	assert.NoError(t, err)

	dbcs = conn.SelectFromContainer(nil)
	assert.Len(t, dbcs, 0)
}

func TestRunContainerOnceWithDockerfile(t *testing.T) {
	t.Parallel()

	store := newTestMock()
	conn := db.New()

	err := store.Set(containerPath, "", 0)
	assert.NoError(t, err)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		self := view.InsertMinion()
		self.Self = true
		self.Role = db.Master
		self.PrivateIP = "leader"
		view.Commit(self)

		etcd := view.InsertEtcd()
		etcd.Leader = true
		view.Commit(etcd)

		dbc := view.InsertContainer()
		dbc.IP = "10.0.0.2"
		dbc.Minion = "1.2.3.4"
		dbc.BlueprintID = "12"
		dbc.Image = "custom"
		dbc.Dockerfile = "dockerfile"
		view.Commit(dbc)

		return nil
	})

	err = runContainerOnce(conn, store)
	assert.NoError(t, err)

	str, err := store.Get(containerPath)
	assert.NoError(t, err)

	expStr := `[
    {
        "IP": "10.0.0.2",
        "Minion": "1.2.3.4",
        "BlueprintID": "12",
        "Created": "0001-01-01T00:00:00Z",
        "Image": "leader:5000/custom"
    }
]`
	assert.Equal(t, expStr, str)
}

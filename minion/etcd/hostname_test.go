package etcd

import (
	"testing"

	"github.com/kelda/kelda/db"
	"github.com/stretchr/testify/assert"
)

func TestRunHostnameOnce(t *testing.T) {
	t.Parallel()

	store := newTestMock()
	conn := db.New()

	err := runHostnameOnce(conn, store)
	assert.Error(t, err)

	err = store.Set(hostnamePath, "", 0)
	assert.NoError(t, err)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		etcd := view.InsertEtcd()
		etcd.Leader = true
		view.Commit(etcd)

		hostname := view.InsertHostname()
		hostname.Hostname = "Robot"
		hostname.IP = "1.2.3.5"
		view.Commit(hostname)
		return nil
	})

	err = runHostnameOnce(conn, store)
	assert.NoError(t, err)

	str, err := store.Get(hostnamePath)
	assert.NoError(t, err)

	expStr := `[
    {
        "Hostname": "Robot",
        "IP": "1.2.3.5"
    }
]`
	assert.Equal(t, expStr, str)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		etcd := view.SelectFromEtcd(nil)[0]
		etcd.Leader = false
		view.Commit(etcd)

		hostname := view.SelectFromHostname(nil)[0]
		hostname.IP = "1.2.3.4"
		view.Commit(hostname)
		return nil
	})

	err = runHostnameOnce(conn, store)
	assert.NoError(t, err)

	expHostname := db.Hostname{
		Hostname: "Robot",
		IP:       "1.2.3.5",
	}
	hostnames := conn.SelectFromHostname(nil)
	assert.Len(t, hostnames, 1)
	hostnames[0].ID = 0
	assert.Equal(t, expHostname, hostnames[0])

	err = runHostnameOnce(conn, store)
	assert.NoError(t, err)

	hostnames = conn.SelectFromHostname(nil)
	assert.Len(t, hostnames, 1)
	hostnames[0].ID = 0
	assert.Equal(t, expHostname, hostnames[0])
}

package minion

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/minion/pb"
)

func TestSetMinionConfig(t *testing.T) {
	t.Parallel()
	s := server{db.New()}

	s.Conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMinion()
		m.Self = true
		m.Role = db.Master
		view.Commit(m)
		return nil
	})

	cfg := pb.MinionConfig{
		PrivateIP:      "priv",
		Blueprint:      "blueprint",
		Provider:       "provider",
		Size:           "size",
		Region:         "region",
		EtcdMembers:    []string{"etcd1", "etcd2"},
		AuthorizedKeys: []string{"key1", "key2"},
	}
	expMinion := db.Minion{
		Self:           true,
		Blueprint:      "blueprint",
		PrivateIP:      "priv",
		Provider:       "provider",
		Role:           db.Master,
		Size:           "size",
		Region:         "region",
		AuthorizedKeys: "key1\nkey2",
	}
	_, err := s.SetMinionConfig(nil, &cfg)
	assert.NoError(t, err)
	checkMinionEquals(t, s.Conn, expMinion)
	checkEtcdEquals(t, s.Conn, db.Etcd{
		EtcdIPs: []string{"etcd1", "etcd2"},
	})

	// Update a field.
	cfg.Blueprint = "new"
	expMinion.Blueprint = "new"
	cfg.EtcdMembers = []string{"etcd3"}
	_, err = s.SetMinionConfig(nil, &cfg)
	assert.NoError(t, err)
	checkMinionEquals(t, s.Conn, expMinion)
	checkEtcdEquals(t, s.Conn, db.Etcd{
		EtcdIPs: []string{"etcd3"},
	})
}

func checkMinionEquals(t *testing.T, conn db.Conn, exp db.Minion) {
	timeout := time.After(1 * time.Second)
	var actual db.Minion
	for {
		actual = conn.MinionSelf()
		actual.ID = 0
		if reflect.DeepEqual(exp, actual) {
			return
		}
		select {
		case <-timeout:
			t.Errorf("Expected minion to be %v, but got %v\n", exp, actual)
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func checkEtcdEquals(t *testing.T, conn db.Conn, exp db.Etcd) {
	timeout := time.After(1 * time.Second)
	var actual db.Etcd
	for {
		conn.Txn(db.AllTables...).Run(func(view db.Database) error {
			actual, _ = view.GetEtcd()
			return nil
		})
		actual.ID = 0
		if reflect.DeepEqual(exp, actual) {
			return
		}
		select {
		case <-timeout:
			t.Errorf("Expected etcd row to be %v, but got %v\n", exp, actual)
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestGetMinionConfig(t *testing.T) {
	t.Parallel()
	s := server{db.New()}

	s.Conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMinion()
		m.Self = true
		m.Blueprint = "selfblueprint"
		m.Role = db.Master
		m.PrivateIP = "selfpriv"
		m.Provider = "selfprovider"
		m.Size = "selfsize"
		m.Region = "selfregion"
		m.AuthorizedKeys = "key1\nkey2"
		view.Commit(m)
		return nil
	})

	// Should only return config for "self".
	s.Conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMinion()
		m.Self = false
		m.Blueprint = "blueprint"
		m.Role = db.Master
		m.PrivateIP = "priv"
		m.Provider = "provider"
		m.Size = "size"
		m.Region = "region"
		m.AuthorizedKeys = "key1\nkey2"
		view.Commit(m)
		return nil
	})
	cfg, err := s.GetMinionConfig(nil, &pb.Request{})
	assert.NoError(t, err)
	assert.Equal(t, pb.MinionConfig{
		Role:           pb.MinionConfig_MASTER,
		PrivateIP:      "selfpriv",
		Blueprint:      "selfblueprint",
		Provider:       "selfprovider",
		Size:           "selfsize",
		Region:         "selfregion",
		AuthorizedKeys: []string{"key1", "key2"},
	}, *cfg)

	// Test returning a full config.
	s.Conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		etcd := view.InsertEtcd()
		etcd.EtcdIPs = []string{"etcd1", "etcd2"}
		view.Commit(etcd)
		return nil
	})
	cfg, err = s.GetMinionConfig(nil, &pb.Request{})
	assert.NoError(t, err)
	assert.Equal(t, pb.MinionConfig{
		Role:           pb.MinionConfig_MASTER,
		PrivateIP:      "selfpriv",
		Blueprint:      "selfblueprint",
		Provider:       "selfprovider",
		Size:           "selfsize",
		Region:         "selfregion",
		EtcdMembers:    []string{"etcd1", "etcd2"},
		AuthorizedKeys: []string{"key1", "key2"},
	}, *cfg)
}

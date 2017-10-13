package client

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kelda/kelda/api"
	"github.com/kelda/kelda/api/client/mocks"
	"github.com/kelda/kelda/connection"
	"github.com/kelda/kelda/db"
)

func TestLeader(t *testing.T) {
	leaderClient := new(mocks.Client)
	newClient = func(host string, _ connection.Credentials) (Client, error) {
		mc := new(mocks.Client)
		mc.On("Close").Return(nil)
		on := mc.On("QueryEtcd")
		switch host {
		case api.RemoteAddress("8.8.8.8"):
			// One machine doesn't know the LeaderIP
			on.Return([]db.Etcd{{LeaderIP: ""}}, nil)
		case api.RemoteAddress("9.9.9.9"):
			// The other machine knows the LeaderIP
			on.Return([]db.Etcd{{LeaderIP: "leader-priv"}}, nil)
		case api.RemoteAddress("leader"):
			return leaderClient, nil
		default:
			t.Fatalf("Unexpected call to getClient with host %s",
				host)
		}

		return mc, nil
	}

	res, err := Leader([]db.Machine{
		{
			PublicIP: "8.8.8.8",
		},
		{
			PublicIP: "9.9.9.9",
		},
		{
			PublicIP:  "leader",
			PrivateIP: "leader-priv",
		},
	}, nil)
	assert.Nil(t, err)
	assert.Equal(t, leaderClient, res)
}

func TestNoLeader(t *testing.T) {
	newClient = func(host string, _ connection.Credentials) (Client, error) {
		mc := new(mocks.Client)
		mc.On("Close").Return(nil)

		// No client knows the leader IP.
		mc.On("QueryEtcd").Return(nil, nil)
		return mc, nil
	}

	_, err := Leader([]db.Machine{
		{
			PublicIP: "8.8.8.8",
		},
		{
			PublicIP: "9.9.9.9",
		},
	}, nil)
	expErr := "no leader found: 8.8.8.8 - no leader information on host 8.8.8.8; " +
		"9.9.9.9 - no leader information on host 9.9.9.9"
	assert.EqualError(t, err, expErr)
}

func TestLeaderNoMachines(t *testing.T) {
	_, err := Leader(nil, nil)
	assert.EqualError(t, err, "no machines to query")
}

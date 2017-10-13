package command

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kelda/kelda/api/client"
	"github.com/kelda/kelda/api/client/mocks"
	"github.com/kelda/kelda/connection"
)

func TestSetupClient(t *testing.T) {
	t.Parallel()

	// Test that we obtain a client, and properly save it.
	expClient := &mocks.Client{}
	newClient := func(host string, _ connection.Credentials) (client.Client, error) {
		assert.Equal(t, "host", host)
		return expClient, nil
	}
	cmd := connectionHelper{
		connectionFlags: connectionFlags{
			host: "host",
		},
	}
	err := cmd.setupClient(newClient)
	assert.NoError(t, err)
	assert.Equal(t, expClient, cmd.client)

	// Test that errors obtaining a client are properly propagated.
	newClient = func(host string, _ connection.Credentials) (client.Client, error) {
		assert.Equal(t, "host", host)
		return nil, assert.AnError
	}
	cmd = connectionHelper{
		connectionFlags: connectionFlags{
			host: "host",
		},
	}
	err = cmd.setupClient(newClient)
	assert.NotNil(t, err)
}

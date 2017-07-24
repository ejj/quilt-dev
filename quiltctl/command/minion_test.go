package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinionFlags(t *testing.T) {
	t.Parallel()

	expRole := "Worker"
	expInboundPubIntf := "inbound"
	expOutboundPubIntf := "outbound"

	cmd := NewMinionCommand()
	err := parseHelper(cmd, []string{"-role", expRole,
		"-inbound-pub-intf", expInboundPubIntf,
		"--outbound-pub-intf", expOutboundPubIntf})

	assert.NoError(t, err)
	assert.Equal(t, expRole, cmd.role)
	assert.Equal(t, expInboundPubIntf, cmd.inboundPubIntf)
	assert.Equal(t, expOutboundPubIntf, cmd.outboundPubIntf)

	// Pass the minion role as an argument instead of a flag.
	expRole = "Master"
	expInboundPubIntf = "inbound"
	expOutboundPubIntf = "outbound"

	cmd = NewMinionCommand()
	err = parseHelper(cmd, []string{
		"-inbound-pub-intf", expInboundPubIntf,
		"--outbound-pub-intf", expOutboundPubIntf,
		expRole})

	assert.NoError(t, err)
	assert.Equal(t, expRole, cmd.role)
	assert.Equal(t, expInboundPubIntf, cmd.inboundPubIntf)
	assert.Equal(t, expOutboundPubIntf, cmd.outboundPubIntf)
}

func TestMinionFailure(t *testing.T) {
	t.Parallel()

	badRole := "Derper"
	cmd := NewMinionCommand()
	roleError := errors.New("no or improper role specified")

	cmd.role = badRole

	assert.Error(t, roleError, cmd.run())
	assert.Equal(t, 1, cmd.Run())

	cmd = NewMinionCommand()
	noRoleError := errors.New("no or improper role specified")

	assert.Error(t, noRoleError, cmd.run())
	assert.Equal(t, 1, cmd.Run())
}

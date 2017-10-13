package plugin

import (
	"errors"
	"testing"

	"github.com/kelda/kelda/minion/ipdef"
	"github.com/kelda/kelda/minion/network/openflow"
	"github.com/kelda/kelda/minion/nl"
	"github.com/kelda/kelda/minion/nl/nlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	dnet "github.com/docker/go-plugins-helpers/network"
)

var (
	zero  = "000000000000000000000000000000000000000000000000"
	one   = "111111111111111111111111111111111111111111111111"
	links = map[string]netlink.Link{}
)

func setup() *nlmock.I {
	mock := new(nlmock.I)
	nl.N = mock
	return mock
}

func TestGetCapabilities(t *testing.T) {
	setup()

	d := driver{}
	resp, err := d.GetCapabilities()
	assert.NoError(t, err)

	exp := dnet.CapabilitiesResponse{Scope: dnet.LocalScope}
	assert.Equal(t, exp, *resp)
}

func TestCreateEndpoint(t *testing.T) {
	mk := setup()

	anErr := errors.New("err")

	vsctl = func(a [][]string) error { return anErr }
	ofctl = func(c openflow.Container) error { return anErr }

	req := &dnet.CreateEndpointRequest{}
	req.EndpointID = zero
	req.Interface = &dnet.EndpointInterface{
		MacAddress: "00:00:00:00:00:00",
	}

	d := driver{}
	_, err := d.CreateEndpoint(req)
	assert.EqualError(t, err, "invalid IP: ")

	req.Interface.Address = "10.1.0.1/8"

	mk.On("AddVeth", mock.Anything, mock.Anything,
		mock.Anything).Once().Return(anErr)
	_, err = d.CreateEndpoint(req)
	assert.EqualError(t, err, "failed to create veth: err")

	mk.On("AddVeth", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mk.On("LinkByName", mock.Anything).Once().Return(nil, anErr)
	_, err = d.CreateEndpoint(req)
	assert.EqualError(t, err, "failed to find link 000000000000000: err")

	link := &netlink.GenericLink{}
	mk.On("LinkByName", mock.Anything).Return(link, nil)
	mk.On("LinkSetUp", link).Once().Return(anErr)
	_, err = d.CreateEndpoint(req)
	assert.EqualError(t, err, "failed to bring up link 000000000000000: err")

	mk.On("LinkSetUp", link).Return(nil)
	req.Interface.MacAddress = ""
	_, err = d.CreateEndpoint(req)
	assert.EqualError(t, err, "ovs-vsctl: err")

	var args [][]string
	vsctl = func(a [][]string) error {
		args = a
		return nil
	}

	expResp := dnet.EndpointInterface{
		MacAddress: ipdef.IPStrToMac("10.1.0.1"),
	}

	resp, err := d.CreateEndpoint(req)
	assert.NoError(t, err)
	assert.Equal(t, expResp, *resp.Interface)
	assert.Equal(t, [][]string{
		{"add-port", "quilt-int", "000000000000000"},
		{"add-port", "quilt-int", "q_0000000000000"},
		{"set", "Interface", "q_0000000000000", "type=patch",
			"options:peer=br_000000000000"},
		{"add-port", "br-int", "br_000000000000"},
		{"set", "Interface", "br_000000000000", "type=patch",
			"options:peer=q_0000000000000",
			"external-ids:attached-mac=02:00:0a:01:00:01",
			"external-ids:iface-id=10.1.0.1"}}, args)

	req.EndpointID = one
	req.Interface.Address = "10.1.0.2/8"
	expResp.MacAddress = ipdef.IPStrToMac("10.1.0.2")
	resp, err = d.CreateEndpoint(req)
	assert.NoError(t, err)
}

func TestDeleteEndpoint(t *testing.T) {
	var args [][]string

	mk := setup()

	vsctl = func(a [][]string) error {
		args = a
		return nil
	}

	link := &netlink.GenericLink{}
	mk.On("LinkByName", "foo").Once().Return(link, nil)
	mk.On("LinkDel", link).Once().Return(nil)

	req := &dnet.DeleteEndpointRequest{EndpointID: "foo"}
	d := driver{}
	err := d.DeleteEndpoint(req)
	assert.NoError(t, err)
	mk.AssertCalled(t, "LinkDel", link)

	expOvsArgs := [][]string{
		{"del-port", "quilt-int", "foo"},
		{"del-port", "quilt-int", "q_foo"},
		{"del-port", "br-int", "br_foo"}}
	assert.Equal(t, expOvsArgs, args)

	mk.On("LinkByName", "foo").Once().Return(nil, errors.New("err"))
	err = d.DeleteEndpoint(req)
	assert.EqualError(t, err, "failed to find link foo: err")

	mk.On("LinkByName", "foo").Return(link, nil)
	mk.On("LinkDel", link).Once().Return(errors.New("err"))
	links["foo"] = &netlink.Dummy{}
	err = d.DeleteEndpoint(req)
	assert.EqualError(t, err, "failed to delete link foo: err")

	vsctl = func(a [][]string) error { return errors.New("err") }
	err = d.DeleteEndpoint(req)
	assert.EqualError(t, err, "ovs-vsctl: err")
}

func TestEndpointInfo(t *testing.T) {
	mk := setup()

	mk.On("LinkByName", "foo").Once().Return(nil, errors.New("err"))

	d := driver{}
	_, err := d.EndpointInfo(&dnet.InfoRequest{EndpointID: "foo"})
	assert.EqualError(t, err, "err")

	mk.On("LinkByName", "foo").Once().Return(nil, nil)
	resp, err := d.EndpointInfo(&dnet.InfoRequest{EndpointID: "foo"})
	assert.NoError(t, err)
	assert.Equal(t, &dnet.InfoResponse{}, resp)
}

func TestJoin(t *testing.T) {
	t.Parallel()

	d := driver{}
	jreq := &dnet.JoinRequest{EndpointID: zero, SandboxKey: "/test/docker0"}
	resp, err := d.Join(jreq)
	assert.NoError(t, err)
	assert.Equal(t, &dnet.JoinResponse{
		InterfaceName: dnet.InterfaceName{
			SrcName:   "tmp_00000000000",
			DstPrefix: "eth"},
		Gateway: "10.0.0.1"}, resp)
}

func TestLeave(t *testing.T) {
	d := driver{}
	_, err := d.Join(&dnet.JoinRequest{EndpointID: zero, SandboxKey: "/test/docker0"})
	assert.NoError(t, err)

	err = d.Leave(&dnet.LeaveRequest{EndpointID: zero})
	assert.NoError(t, err)
}

package network

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/minion/ipdef"
	"github.com/kelda/kelda/minion/ovsdb"
	"github.com/kelda/kelda/minion/ovsdb/mocks"
)

func TestUpdateLogicalSwitch(t *testing.T) {
	t.Parallel()

	containers := []db.Container{{IP: "1.2.3.4"}}
	anErr := errors.New("err")
	client := new(mocks.Client)

	client.On("LogicalSwitchExists", lSwitch).Return(true, nil)
	client.On("ListSwitchPorts").Return(nil, anErr).Once()
	updateLogicalSwitch(client, containers)
	client.AssertNotCalled(t, "CreateSwitchPort")
	client.AssertNotCalled(t, "DeleteSwitchPort")

	client.On("CreateSwitchPort", lSwitch, ovsdb.SwitchPort{
		Name: loadBalancerSwitchPort,
		Type: "router",
		Options: map[string]string{
			"router-port": loadBalancerRouterPort,
		},
	}).Return(nil)

	client.On("ListSwitchPorts").Return([]ovsdb.SwitchPort{{Name: "1.2.3.5"}}, nil)
	client.On("DeleteSwitchPort", lSwitch, ovsdb.SwitchPort{
		Name: "1.2.3.5", Addresses: nil}).Return(anErr).Once()
	client.On("CreateSwitchPort", lSwitch, ovsdb.SwitchPort{
		Name:      "1.2.3.4",
		Addresses: []string{"02:00:01:02:03:04 1.2.3.4"},
	}).Return(anErr).Once()
	updateLogicalSwitch(client, containers)
	client.AssertExpectations(t)

	client.On("DeleteSwitchPort", lSwitch, ovsdb.SwitchPort{
		Name: "1.2.3.5", Addresses: []string(nil)}).Return(nil)
	client.On("CreateSwitchPort", lSwitch, ovsdb.SwitchPort{
		Name:      "1.2.3.4",
		Addresses: []string{"02:00:01:02:03:04 1.2.3.4"},
	}).Return(nil).Once()
	updateLogicalSwitch(client, containers)
	client.AssertExpectations(t)
}

func TestCreateLogicalSwitch(t *testing.T) {
	t.Parallel()

	client := new(mocks.Client)
	client.On("ListSwitchPorts").Return(nil, nil)
	client.On("CreateSwitchPort", mock.Anything, mock.Anything).Return(nil)

	client.On("LogicalSwitchExists", lSwitch).Return(false, assert.AnError).Once()
	updateLogicalSwitch(client, nil)
	client.AssertNotCalled(t, "CreateLogicalSwitch", mock.Anything)

	client.On("LogicalSwitchExists", lSwitch).Return(true, nil).Once()
	updateLogicalSwitch(client, nil)
	client.AssertNotCalled(t, "CreateLogicalSwitch", mock.Anything)

	client.On("CreateLogicalSwitch", lSwitch).Return(nil).Once()
	client.On("LogicalSwitchExists", lSwitch).Return(false, nil).Once()
	updateLogicalSwitch(client, nil)
	client.AssertCalled(t, "CreateLogicalSwitch", lSwitch)

	client.On("CreateLogicalSwitch", lSwitch).Return(assert.AnError).Once()
	client.On("LogicalSwitchExists", lSwitch).Return(false, nil).Once()
	updateLogicalSwitch(client, nil)
	client.AssertNotCalled(t, "ListSwitchPorts", lSwitch)
}

func TestCreateLogicalRouter(t *testing.T) {
	t.Parallel()

	client := new(mocks.Client)
	client.On("ListRouterPorts").Return(nil, nil)
	client.On("CreateRouterPort", mock.Anything, mock.Anything).Return(nil)

	client.On("LogicalRouterExists", loadBalancerRouter).Return(
		false, assert.AnError).Once()
	updateLoadBalancerRouter(client)
	client.AssertNotCalled(t, "CreateLogicalRouter", mock.Anything)

	client.On("LogicalRouterExists", loadBalancerRouter).Return(true, nil).Once()
	updateLoadBalancerRouter(client)
	client.AssertNotCalled(t, "CreateLogicalRouter", mock.Anything)

	client.On("CreateLogicalRouter", loadBalancerRouter).Return(nil).Once()
	client.On("LogicalRouterExists", loadBalancerRouter).Return(false, nil).Once()
	updateLoadBalancerRouter(client)
	client.AssertCalled(t, "CreateLogicalRouter", loadBalancerRouter)

	client.On("CreateLogicalRouter", loadBalancerRouter).Return(assert.AnError).Once()
	client.On("LogicalRouterExists", loadBalancerRouter).Return(false, nil).Once()
	updateLoadBalancerRouter(client)
	client.AssertNotCalled(t, "ListRouterPorts", loadBalancerRouter)
}

func TestUpdateLoadBalancerRouter(t *testing.T) {
	t.Parallel()

	client := new(mocks.Client)

	client.On("LogicalRouterExists", loadBalancerRouter).Return(true, nil)
	client.On("ListRouterPorts").Return([]ovsdb.RouterPort{{Name: "toDelete"}}, nil)
	client.On("DeleteRouterPort", loadBalancerRouter, ovsdb.RouterPort{
		Name: "toDelete",
	}).Return(nil).Once()
	client.On("CreateRouterPort", loadBalancerRouter, ovsdb.RouterPort{
		Name:     loadBalancerRouterPort,
		MAC:      ipdef.LoadBalancerMac,
		Networks: []string{ipdef.QuiltSubnet.String()},
	}).Return(nil).Once()
	updateLoadBalancerRouter(client)
	client.AssertExpectations(t)
}

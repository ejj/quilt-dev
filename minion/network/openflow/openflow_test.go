package openflow

import (
	"errors"
	"testing"

	"github.com/quilt/quilt/minion/ovsdb"
	"github.com/quilt/quilt/minion/ovsdb/mocks"
	"github.com/stretchr/testify/assert"
)

func TestAddReplaceFlows(t *testing.T) {
	anErr := errors.New("err")
	ovsdb.Open = func() (ovsdb.Client, error) { return nil, anErr }
	assert.EqualError(t, ReplaceFlows(nil), "ovsdb-server connection: err")
	assert.EqualError(t, AddFlows(nil), "ovsdb-server connection: err")

	client := new(mocks.Client)
	ovsdb.Open = func() (ovsdb.Client, error) {
		return client, nil
	}

	actionsToFlows := map[string][]string{}
	diffFlowsShouldErr := true
	ofctl = func(a string, f []string) error {
		actionsToFlows[a] = f
		if a == "diff-flows" && diffFlowsShouldErr {
			return errors.New("flows differ")
		}
		return nil
	}

	client.On("Disconnect").Return(nil)
	client.On("OpenFlowPorts").Return(map[string]int{}, nil)
	assert.NoError(t, ReplaceFlows(nil))
	client.AssertCalled(t, "Disconnect")
	client.AssertCalled(t, "OpenFlowPorts")
	assert.Equal(t, map[string][]string{
		"diff-flows":    allFlows(nil),
		"replace-flows": allFlows(nil),
	}, actionsToFlows)

	// Test that we don't call replace-flows when there are no differences.
	actionsToFlows = map[string][]string{}
	diffFlowsShouldErr = false
	assert.NoError(t, ReplaceFlows(nil))
	assert.Equal(t, map[string][]string{
		"diff-flows": allFlows(nil),
	}, actionsToFlows)

	actionsToFlows = map[string][]string{}
	assert.NoError(t, AddFlows(nil))
	client.AssertCalled(t, "Disconnect")
	client.AssertCalled(t, "OpenFlowPorts")

	assert.Equal(t, map[string][]string{
		"add-flows": nil,
	}, actionsToFlows)

	ofctl = func(a string, f []string) error { return anErr }
	assert.EqualError(t, ReplaceFlows(nil), "ovs-ofctl: err")
	client.AssertCalled(t, "Disconnect")
	client.AssertCalled(t, "OpenFlowPorts")

	assert.EqualError(t, AddFlows(nil), "ovs-ofctl: err")
	client.AssertCalled(t, "Disconnect")
	client.AssertCalled(t, "OpenFlowPorts")
}

func TestAllFlows(t *testing.T) {
	t.Parallel()
	flows := allFlows([]container{
		{patch: 4, veth: 5, mac: "66:66:66:66:66:66"},
		{patch: 9, veth: 8, mac: "99:99:99:99:99:99"}})
	exp := append(staticFlows,
		"table=0,in_port=5,dl_src=66:66:66:66:66:66,"+
			"actions=load:0x4->NXM_NX_REG0[],resubmit(,1)",
		"table=0,in_port=4,actions=output:5",
		"table=2,priority=900,dl_dst=66:66:66:66:66:66,action=output:5",
		"table=0,in_port=8,dl_src=99:99:99:99:99:99,"+
			"actions=load:0x9->NXM_NX_REG0[],resubmit(,1)",
		"table=0,in_port=9,actions=output:8",
		"table=2,priority=900,dl_dst=99:99:99:99:99:99,action=output:8",
		"table=2,priority=1000,dl_dst=ff:ff:ff:ff:ff:ff,"+
			"actions=output:5,output:8")
	assert.Equal(t, exp, flows)
}

func TestResolveContainers(t *testing.T) {
	t.Parallel()

	res := resolveContainers(map[string]int{"a": 3, "b": 4}, []Container{
		{Veth: "a", Patch: "b", Mac: "mac"},
		{Veth: "c", Patch: "d", Mac: "mac2"}})
	assert.Equal(t, []container{{veth: 3, patch: 4, mac: "mac"}}, res)
}

package command

import (
	"testing"

	clientMock "github.com/kelda/kelda/api/client/mocks"
	"github.com/kelda/kelda/blueprint"
	"github.com/kelda/kelda/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStopNamespaceDefault(t *testing.T) {
	t.Parallel()

	c := new(clientMock.Client)
	c.On("QueryBlueprints").Once().Return([]db.Blueprint{{
		Blueprint: blueprint.Blueprint{Namespace: "testSpace"}}}, nil)
	c.On("Deploy", mock.Anything).Return(nil)

	stopCmd := NewStopCommand()
	stopCmd.client = c
	stopCmd.Run()

	c.AssertCalled(t, "Deploy", blueprint.Blueprint{Namespace: "testSpace"}.String())

	c.On("QueryBlueprints").Return(nil, nil)
	assert.Equal(t, 1, stopCmd.Run(),
		"can't retrieve namespace if no cluster is deployed")
}

func TestStopNamespace(t *testing.T) {
	t.Parallel()

	c := &clientMock.Client{}
	c.On("QueryBlueprints").Return(nil, nil)
	c.On("Deploy", mock.Anything).Return(nil)

	stopCmd := NewStopCommand()
	stopCmd.client = c
	stopCmd.namespace = "namespace"
	stopCmd.Run()

	c.AssertCalled(t, "Deploy", blueprint.Blueprint{Namespace: "namespace"}.String())
}

func TestStopContainers(t *testing.T) {
	t.Parallel()

	c := &clientMock.Client{}
	c.On("QueryBlueprints").Return([]db.Blueprint{{
		Blueprint: blueprint.Blueprint{
			Namespace: "testSpace",
			Machines: []blueprint.Machine{
				{Provider: "Amazon"},
				{Provider: "Google"}}},
	}}, nil)

	c.On("Deploy", mock.Anything).Return(nil)

	stopCmd := NewStopCommand()
	stopCmd.client = c
	stopCmd.onlyContainers = true
	stopCmd.Run()

	c.AssertCalled(t, "Deploy", blueprint.Blueprint{
		Namespace: "testSpace",
		Machines: []blueprint.Machine{{
			Provider: "Amazon",
		}, {
			Provider: "Google",
		}}}.String())

}

func TestStopFlags(t *testing.T) {
	t.Parallel()

	expNamespace := "namespace"
	checkStopParsing(t, []string{"-namespace", expNamespace}, expNamespace, nil)
	checkStopParsing(t, []string{expNamespace}, expNamespace, nil)
	checkStopParsing(t, []string{}, "", nil)
}

func checkStopParsing(t *testing.T, args []string, expNamespace string, expErr error) {
	stopCmd := NewStopCommand()
	err := parseHelper(stopCmd, args)

	assert.Equal(t, expErr, err)
	assert.Equal(t, expNamespace, stopCmd.namespace)
}

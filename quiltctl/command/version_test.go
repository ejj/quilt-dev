package command

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quilt/quilt/api/client/mocks"
)

func TestVersionFlags(t *testing.T) {
	t.Parallel()

	expHost := "mockHost"

	cmd := NewVersionCommand()
	err := parseHelper(cmd, []string{"-H", expHost})

	assert.NoError(t, err)
	assert.Equal(t, expHost, cmd.host)
}

func TestGetDaemonVersion(t *testing.T) {
	t.Parallel()

	mockLocalClient := &mocks.Client{}
	mockLocalClient.On("Version").Once().Return("mockVersion", nil)
	mockLocalClient.On("Close").Return(nil)

	vCmd := Version{
		connectionHelper: connectionHelper{client: mockLocalClient},
	}

	res := vCmd.Run()
	assert.Zero(t, res)

	mockLocalClient.On("Version").Return("", assert.AnError)
	res = vCmd.Run()
	assert.NotZero(t, res)
}

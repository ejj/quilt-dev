package vagrant

import (
	"testing"

	"github.com/kelda/kelda/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestVagrantFile(t *testing.T) {
	vagrantTemplate = "({{.CloudConfigPath}}) ({{.Box}}) ({{.BoxVersion}}) " +
		"({{.SizePath}})"

	box = "testBox"
	boxVersion = "testVersion"

	res := createVagrantFile()
	exp := "(/user-data) (testBox) (testVersion) (/size)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}
}

func TestInitMachine(t *testing.T) {
	util.AppFs = afero.NewMemMapFs()

	vagrantTemplate = "({{.CloudConfigPath}}) ({{.Box}}) ({{.BoxVersion}}) " +
		"({{.SizePath}})"
	cloudConfig := "testCloudConfig"
	size := "2,2"
	id := "testing"

	initMachine(cloudConfig, size, id)

	vdir, err := vagrantDir()

	assert.Nil(t, err)

	path := vdir + id

	resCloudConfig, err := util.ReadFile(path + cloudConfigPath)
	assert.Nil(t, err)
	assert.Equal(t, cloudConfig, resCloudConfig)

	resSize, err := util.ReadFile(path + sizePath)
	assert.Nil(t, err)
	assert.Equal(t, size, resSize)

	resVagrantFile, err := util.ReadFile(path + vagrantFilePath)
	assert.Nil(t, err)
	expFile := createVagrantFile()
	assert.Equal(t, expFile, resVagrantFile)
}

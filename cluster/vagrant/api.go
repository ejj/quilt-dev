package vagrant

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/quilt/quilt/util"
)

const inboundPublicInterface = "enp0s8"
const vagrantCmd = "vagrant"
const shCmd = "sh"
const cloudConfigPath = "/user-data"
const sizePath = "/size"
const vagrantFilePath = "/Vagrantfile"

// Allow mocking out for unit tests
var box = "ubuntu/xenial64"
var boxVersion = "20170515.0.0"

// createVagrantFile generates a VagrantFile for the machine.
func createVagrantFile() string {
	t := template.Must(template.New("VagrantFile").Parse(vagrantTemplate))

	var vagrantFileBytes bytes.Buffer
	err := t.Execute(&vagrantFileBytes, struct {
		CloudConfigPath string
		Box             string
		BoxVersion      string
		SizePath        string
	}{
		CloudConfigPath: cloudConfigPath,
		Box:             box,
		BoxVersion:      boxVersion,
		SizePath:        sizePath,
	})

	if err != nil {
		panic(err)
	}

	return vagrantFileBytes.String()
}

// initMachine creates the files necessary to initialize a vagrant machine.
func initMachine(cloudConfig string, size string, id string) error {
	c.Inc("Initialize Machine")
	vdir, err := vagrantDir()
	if err != nil {
		return err
	}

	if _, err := util.AppFs.Stat(vdir); os.IsNotExist(err) {
		util.AppFs.Mkdir(vdir, os.ModeDir|os.ModePerm)
	}

	path := vdir + id
	util.AppFs.Mkdir(path, os.ModeDir|os.ModePerm)

	err = util.WriteFile(path+cloudConfigPath, []byte(cloudConfig), 0644)
	if err != nil {
		destroy(id)
		return err
	}

	vagrantFile := createVagrantFile()

	err = util.WriteFile(path+vagrantFilePath, []byte(vagrantFile), 0644)
	if err != nil {
		destroy(id)
		return err
	}

	err = util.WriteFile(path+sizePath, []byte(size), 0644)
	if err != nil {
		destroy(id)
		return err
	}

	return nil
}

func up(id string) error {
	c.Inc("Up")
	_, stderr, err := shell(id, `vagrant --machine-readable up`)
	if err != nil {
		log.Errorf("Failed to start Vagrant machine: %s", string(stderr))
		return errors.New("unable to start machine")
	}
	return nil
}

func destroy(id string) error {
	c.Inc("Destroy")
	_, stderr, err := shell(id,
		`vagrant --machine-readable destroy -f; cd ../; rm -rf %s`)
	if err != nil {
		log.Errorf("Failed to destroy Vagrant machine: %s", string(stderr))
		return errors.New("unable to destroy machine")
	}
	return nil
}

func publicIP(id string) (string, error) {
	c.Inc("Get Public IP")
	ip, stderr, err := shell(id, fmt.Sprintf(
		`vagrant ssh -c "ip -f inet addr show %s | grep -Po 'inet \K[\d.]+'"`,
		inboundPublicInterface))
	if err != nil {
		log.Errorf("Failed to parse Vagrant machine IP: %s", string(stderr))
		return "", err
	}
	return strings.TrimSuffix(string(ip), "\n"), nil
}

func status(id string) (string, error) {
	c.Inc("Status")
	output, stderr, err := shell(id, `vagrant --machine-readable status`)
	if err != nil {
		log.Errorf("Failed to retrieve Vagrant machine status: %s",
			string(stderr))
		return "", errors.New("unable to retrieve machine status")
	}
	lines := bytes.Split(output, []byte("\n"))
	for _, line := range lines {
		words := strings.Split(string(line[:]), ",")
		if len(words) >= 4 {
			if strings.Compare(words[2], "state") == 0 {
				return words[3], nil
			}
		}
	}
	return "", nil
}

func list() ([]string, error) {
	c.Inc("List")
	subdirs := []string{}
	vdir, err := vagrantDir()
	if err != nil {
		return nil, err
	}
	if _, err := util.AppFs.Stat(vdir); os.IsNotExist(err) {
		return subdirs, nil
	}

	files, err := ioutil.ReadDir(vdir)
	if err != nil {
		return subdirs, err
	}
	for _, file := range files {
		subdirs = append(subdirs, file.Name())
	}
	return subdirs, nil
}

func addBox(name string, provider string) error {
	/* Adding a box fails if it already exists, hence the check. */
	c.Inc("Box Add")
	exists, err := containsBox(name)
	if err == nil && exists {
		return nil
	}
	err = exec.Command(vagrantCmd, []string{"--machine-readable", "box", "add",
		"--provider", provider, name, "--box-version", boxVersion}...).Run()
	if err != nil {
		return errors.New("unable to add box")
	}
	return nil
}

func containsBox(name string) (bool, error) {
	c.Inc("Box List")
	output, err := exec.Command(vagrantCmd, []string{"--machine-readable", "box",
		"list"}...).Output()
	if err != nil {
		return false, errors.New("unable to list machines")
	}
	lines := bytes.Split(output, []byte("\n"))
	for _, line := range lines {
		words := strings.Split(string(line[:]), ",")
		if words[len(words)-1] == name {
			return true, nil
		}
	}
	return false, nil
}

func shell(id string, commands string) ([]byte, []byte, error) {
	c.Inc("Shell")
	chdir := `(cd %s; `
	vdir, err := vagrantDir()
	if err != nil {
		return nil, nil, err
	}
	chdir = fmt.Sprintf(chdir, vdir+id)
	shellCommand := chdir + strings.Replace(commands, "%s", id, -1) + ")"

	var outbuf, errbuf bytes.Buffer
	cmd := exec.Command(shCmd, []string{"-c", shellCommand}...)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err = cmd.Run()

	return outbuf.Bytes(), errbuf.Bytes(), err
}

func vagrantDir() (string, error) {
	dir, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	vagrantDir := dir + "/.vagrant/"
	return vagrantDir, nil
}

func size(id string) string {
	size, _, err := shell(id, "cat size")
	if err != nil {
		return ""
	}
	return string(size)
}

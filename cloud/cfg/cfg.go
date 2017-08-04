package cfg

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/version"

	log "github.com/Sirupsen/logrus"
)

const (
	quiltImage = "quilt/quilt"
)

// Allow mocking out for the unit tests.
var ver = version.Version

// MinionTLSDir is where minions should look for their TLS configuration on boot.
var MinionTLSDir string

// Ubuntu generates a cloud config file for the Ubuntu operating system with the
// corresponding `version`.
func Ubuntu(m db.Machine, inboundPublic string) string {
	t := template.Must(template.New("cloudConfig").Parse(cfgTemplate))

	img := fmt.Sprintf("%s:%s", quiltImage, ver)

	dockerOpts := ""
	if MinionTLSDir != "" {
		// Mount the TLSDir as a read-only host volume. This is necessary for
		// the minion container to access the TLS certificates copied by
		// the daemon onto the host machine.
		dockerOpts = fmt.Sprintf("-v %[1]s:%[1]s:ro", MinionTLSDir)
	}

	var cloudConfigBytes bytes.Buffer
	err := t.Execute(&cloudConfigBytes, struct {
		QuiltImage string
		SSHKeys    string
		LogLevel   string
		MinionOpts string
		DockerOpts string
	}{
		QuiltImage: img,
		SSHKeys:    strings.Join(m.SSHKeys, "\n"),
		LogLevel:   log.GetLevel().String(),
		MinionOpts: minionOptions(m.Role, inboundPublic, MinionTLSDir),
		DockerOpts: dockerOpts,
	})
	if err != nil {
		panic(err)
	}

	return cloudConfigBytes.String()
}

func minionOptions(role db.Role, inboundPublic, tlsDir string) string {
	options := fmt.Sprintf("--role %q", role)

	if inboundPublic != "" {
		options += fmt.Sprintf(" --inbound-pub-intf %q", inboundPublic)
	}

	if tlsDir != "" {
		options += fmt.Sprintf(" --tls-dir %q", tlsDir)
	}

	return options
}

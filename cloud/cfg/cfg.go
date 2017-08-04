package cfg

import (
	"bytes"
	"fmt"
	"sort"
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

// Ubuntu generates a cloud config file for the Ubuntu operating system with the
// corresponding `version`.
func Ubuntu(opts Options) string {
	t := template.Must(template.New("cloudConfig").Parse(cfgTemplate))

	img := fmt.Sprintf("%s:%s", quiltImage, ver)

	dockerOpts := ""
	if opts.MinionOpts.TLSDir != "" {
		// Mount the TLSDir as a read-only host volume. This is necessary for
		// the minion container to access the TLS certificates copied by
		// the daemon onto the host machine.
		dockerOpts = fmt.Sprintf("-v %[1]s:%[1]s:ro", opts.MinionOpts.TLSDir)
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
		SSHKeys:    strings.Join(opts.SSHKeys, "\n"),
		LogLevel:   log.GetLevel().String(),
		MinionOpts: opts.MinionOpts.String(),
		DockerOpts: dockerOpts,
	})
	if err != nil {
		panic(err)
	}

	return cloudConfigBytes.String()
}

// Options defines configuration for the cloud config.
type Options struct {
	SSHKeys    []string
	MinionOpts MinionOptions
}

// MinionOptions defines the command line flags the minion should be invoked with.
type MinionOptions struct {
	Role            db.Role
	InboundPubIntf  string
	OutboundPubIntf string
	TLSDir          string
}

func (opts MinionOptions) String() string {
	optsMap := map[string]string{
		"role":              string(opts.Role),
		"inbound-pub-intf":  opts.InboundPubIntf,
		"outbound-pub-intf": opts.OutboundPubIntf,
		"tls-dir":           opts.TLSDir,
	}

	// Sort the option keys so that the command line arguments are consistently
	// formatted. This is helpful for unit testing the output.
	var optsKeys []string
	for key := range optsMap {
		optsKeys = append(optsKeys, key)
	}
	sort.Strings(optsKeys)

	var optsList []string
	for _, name := range optsKeys {
		if val := optsMap[name]; val != "" {
			optsList = append(optsList, fmt.Sprintf("--%s %q", name, val))
		}
	}

	return strings.Join(optsList, " ")
}

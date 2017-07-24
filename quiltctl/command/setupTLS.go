package command

import (
	"errors"
	"flag"
	"net"

	tlsIO "github.com/quilt/quilt/connection/credentials/tls/io"
	"github.com/quilt/quilt/connection/credentials/tls/rsa"
	"github.com/quilt/quilt/util"

	log "github.com/Sirupsen/logrus"
)

// SetupTLS contains the options for setting up Quilt TLS.
type SetupTLS struct {
	outDir string
}

const setupTLSCommands = `quilt setup-tls [OPTIONS] OUT_DIR`
const setupTLSExplanation = `Create the files necessary for TLS-encrypted communication
with Quilt.  It generates private keys and certs for the signing CA, and peers.`

// InstallFlags sets up flag parsing for the SetupTLS command.
func (sCmd *SetupTLS) InstallFlags(flags *flag.FlagSet) {
	flags.StringVar(&sCmd.outDir, "outDir", "",
		"the directory to write the certificates")

	flags.Usage = func() {
		util.PrintUsageString(setupTLSCommands, setupTLSExplanation, flags)
	}
}

// Parse parses the command line arguments for the setupTLS command.
func (sCmd *SetupTLS) Parse(args []string) error {
	if sCmd.outDir == "" {
		if len(args) == 0 {
			return errors.New("no output directory specified")
		}
		sCmd.outDir = args[0]
	}

	return nil
}

// BeforeRun makes any necessary post-parsing transformations.
func (sCmd *SetupTLS) BeforeRun() error {
	return nil
}

// AfterRun performs any necessary post-run cleanup.
func (sCmd *SetupTLS) AfterRun() error {
	return nil
}

// Run creates the TLS configuration.
func (sCmd *SetupTLS) Run() int {
	if err := util.AppFs.Mkdir(sCmd.outDir, 0700); err != nil {
		log.WithError(err).Error("Failed to create output directory")
		return 1
	}

	ca, err := rsa.NewCertificateAuthority()
	if err != nil {
		log.WithError(err).Error("Unable to create CA")
		return 1
	}

	// Generate a signed certificate for use by the Daemon server, and client
	// connections. We don't know what IP this certificate will be used on, and
	// it doesn't matter (because the Daemon server listens on a Unix socket, and
	// the host is ignored on client connections), so we arbitrarily create it
	// for 0.0.0.0.
	signed, err := rsa.NewSigned(ca, net.IP{0, 0, 0, 0})
	if err != nil {
		log.WithError(err).Error("Unable to create signed key pair")
		return 1
	}

	for _, f := range tlsIO.DaemonFiles(sCmd.outDir, ca, signed) {
		if err := util.WriteFile(f.Path, []byte(f.Content), f.Mode); err != nil {
			log.WithError(err).WithField("path", f.Path).
				Error("Unable to write file")
			return 1
		}
	}

	return 0
}

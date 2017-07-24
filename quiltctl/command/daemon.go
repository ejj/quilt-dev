package command

import (
	"crypto/rand"
	goRSA "crypto/rsa"
	"encoding/base64"
	"flag"
	"fmt"

	"golang.org/x/crypto/ssh"

	"github.com/quilt/quilt/api/server"
	"github.com/quilt/quilt/cluster"
	"github.com/quilt/quilt/connection/credentials/tls"
	tlsIO "github.com/quilt/quilt/connection/credentials/tls/io"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/engine"
	"github.com/quilt/quilt/quiltctl/command/credentials"
	"github.com/quilt/quilt/util"
	"github.com/quilt/quilt/version"

	log "github.com/Sirupsen/logrus"
)

// Daemon contains the options for running the Quilt daemon.
type Daemon struct {
	adminSSHPrivateKey string

	*connectionFlags
}

// NewDaemonCommand creates a new Daemon command instance.
func NewDaemonCommand() *Daemon {
	return &Daemon{
		connectionFlags: &connectionFlags{},
	}
}

var daemonCommands = "quilt daemon [-H=<daemon_host>]"
var daemonExplanation = "Start the quilt daemon, which listens for quilt API requests."

// InstallFlags sets up parsing for command line flags
func (dCmd *Daemon) InstallFlags(flags *flag.FlagSet) {
	dCmd.connectionFlags.InstallFlags(flags)
	flags.StringVar(&dCmd.adminSSHPrivateKey, "admin-ssh-private-key", "",
		"if specified, all machines will be configured to allow access from "+
			"this private SSH key")
	flags.Usage = func() {
		util.PrintUsageString(daemonCommands, daemonExplanation, flags)
	}
}

// Parse parses the command line arguments for the daemon command.
func (dCmd *Daemon) Parse(args []string) error {
	return nil
}

// BeforeRun makes any necessary post-parsing transformations.
func (dCmd *Daemon) BeforeRun() error {
	return nil
}

// AfterRun performs any necessary post-run cleanup.
func (dCmd *Daemon) AfterRun() error {
	return nil
}

// Run starts the daemon.
func (dCmd *Daemon) Run() int {
	log.WithField("version", version.Version).Info("Starting Quilt daemon")

	var sshKey ssh.Signer
	if dCmd.adminSSHPrivateKey != "" {
		var err error
		sshKey, err = parseSSHPrivateKey(dCmd.adminSSHPrivateKey)
		if err != nil {
			log.WithError(err).Errorf(
				"Failed to parse private key %s", dCmd.adminSSHPrivateKey)
			return 1
		}
	}

	if sshKey == nil && dCmd.tlsDir != "" {
		log.Info("No admin key supplied, but TLS is enabled, which requires an " +
			"admin SSH key to copy TLS credentials to the cluster. " +
			"Auto-generating an in-memory key.")
		var err error
		sshKey, err = newSSHPrivateKey()
		if err != nil {
			log.WithError(err).Error("Failed to generate SSH key")
			return 1
		}
	}

	creds, err := credentials.Read(dCmd.tlsDir)
	if err != nil {
		log.WithError(err).Error("Failed to parse credentials. " +
			"Did you run `quilt setup-tls` to generate TLS credentials?")
		return 1
	}

	conn := db.New()
	go engine.Run(conn, getPublicKey(sshKey))
	go server.Run(conn, dCmd.host, true, creds)

	var minionTLSDir string
	if _, isTLS := creds.(tls.TLS); isTLS {
		minionTLSDir = "/home/quilt/.quilt/tls"

		ca, err := tlsIO.ReadCA(dCmd.tlsDir)
		if err != nil {
			log.WithError(err).Error("Failed to parse certificate authority")
			return 1
		}

		go cluster.SyncCredentials(conn, minionTLSDir, sshKey, ca)
	}

	cluster.Run(conn, creds, minionTLSDir)
	return 0
}

func newSSHPrivateKey() (ssh.Signer, error) {
	key, err := goRSA.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return ssh.NewSignerFromKey(key)
}

func parseSSHPrivateKey(path string) (ssh.Signer, error) {
	keyStr, err := util.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %s", err)
	}

	return ssh.ParsePrivateKey([]byte(keyStr))
}

func getPublicKey(sshPrivKey ssh.Signer) string {
	if sshPrivKey == nil {
		return ""
	}
	pubKey := base64.StdEncoding.EncodeToString(sshPrivKey.PublicKey().Marshal())
	pubKeyType := sshPrivKey.PublicKey().Type()
	return pubKeyType + " " + pubKey
}

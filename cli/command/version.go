package command

import (
	"flag"
	"fmt"

	"github.com/kelda/kelda/util"
	"github.com/kelda/kelda/version"

	log "github.com/sirupsen/logrus"
)

// Version prints the Quilt version information.
type Version struct {
	connectionHelper
}

// NewVersionCommand creates a new Version command instance.
func NewVersionCommand() *Version {
	return &Version{}
}

var versionCommands = "quilt version [OPTIONS]"
var versionExplanation = "Show the Quilt version information."

// InstallFlags sets up parsing for command line flags.
func (vCmd *Version) InstallFlags(flags *flag.FlagSet) {
	vCmd.connectionHelper.InstallFlags(flags)
	flags.Usage = func() {
		util.PrintUsageString(versionCommands, versionExplanation, flags)
	}
}

// Parse parses the command line arguments for the version command.
func (vCmd *Version) Parse(args []string) error {
	return nil
}

// Run prints the version information.
func (vCmd *Version) Run() int {
	fmt.Println("Client:", version.Version)

	daemonVersion, err := vCmd.client.Version()
	if err != nil {
		log.WithError(err).Error("Failed to get daemon version")
		return 1
	}
	fmt.Println("Daemon:", daemonVersion)

	return 0
}

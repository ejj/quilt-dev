package command

import (
	"flag"
	"os"
	"os/exec"

	"github.com/kelda/kelda/util"

	log "github.com/sirupsen/logrus"
)

// Init represents an Init command.
type Init struct{}

var initCommands = `quilt init`

var initExplanation = `Create a new infrastructure to use with
baseInfrastructure().

After creating an infrastructure named 'infra', the infrastructure can be used
in any blueprint by calling baseInfrastructure(quilt, 'infra').`

// InstallFlags sets up parsing for command line flags.
func (iCmd *Init) InstallFlags(flags *flag.FlagSet) {
	flags.Usage = func() {
		util.PrintUsageString(initCommands, initExplanation, flags)
	}
}

// Parse parses the command line arguments for the stop command.
func (iCmd *Init) Parse(args []string) error {
	return nil
}

// BeforeRun makes any necessary post-parsing transformations.
func (iCmd *Init) BeforeRun() error {
	return nil
}

// AfterRun performs any necessary post-run cleanup.
func (iCmd *Init) AfterRun() error {
	return nil
}

// Run executes the Nodejs initializer that prompts the user.
func (iCmd *Init) Run() int {
	// Assumes `cli/command/init/intializer.js` was installed in the path
	// somewhere as `quilt-initializer.js`. This is done automatically for users by
	// npm when installed.
	executable := "quilt-initializer.js"
	cmd := exec.Command(executable)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if _, err := exec.LookPath(executable); err != nil {
		log.Errorf("%s: Make sure that "+
			"cli/command/init/intializer.js is installed in your $PATH as "+
			"%s. This is done automatically with "+
			"`npm install -g @quilt/install`, but if you're running Quilt "+
			"from source, you must set up the symlink manually. You can "+
			"do this by executing `ln -s <ABS_PATH_TO_QUILT_SOURCE>/"+
			"cli/command/init/initializer.js /usr/local/bin/%s`",
			err, executable, executable)
		return 1
	}

	if err := cmd.Run(); err != nil {
		return 1
	}

	return 0
}

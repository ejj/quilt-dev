package quiltctl

import (
	"flag"
	"os"

	"github.com/quilt/quilt/quiltctl/command"

	log "github.com/Sirupsen/logrus"
)

// Note the `minion` command is in quiltclt_posix.go as it only runs on posix systems.
var commands = map[string]command.SubCommand{
	"daemon":     command.NewDaemonCommand(),
	"inspect":    &command.Inspect{},
	"logs":       command.NewLogCommand(),
	"ps":         command.NewPsCommand(),
	"run":        command.NewRunCommand(),
	"ssh":        command.NewSSHCommand(),
	"stop":       command.NewStopCommand(),
	"version":    command.NewVersionCommand(),
	"debug-logs": command.NewDebugCommand(),
	"counters":   &command.Counters{},
}

// Run parses and runs the quiltctl subcommand given the command line arguments.
func Run(cmdName string, args []string) {
	cmd, err := parseSubcommand(cmdName, commands[cmdName], args)
	if err != nil {
		log.WithError(err).Error("Unable to parse subcommand.")
		os.Exit(1)
	}

	if err := cmd.BeforeRun(); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	exitCode := cmd.Run()
	if err := cmd.AfterRun(); err != nil {
		log.Error(err)
		// The exit code returned by `Run` has precedence if both `Run` and
		// `AfterRun` error.
		if exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// HasSubcommand returns true if quiltctl has a subcommand for the given name.
func HasSubcommand(name string) bool {
	_, ok := commands[name]
	return ok
}

func parseSubcommand(name string, cmd command.SubCommand, args []string) (
	command.SubCommand, error) {

	flags := flag.NewFlagSet(name, flag.ExitOnError)
	cmd.InstallFlags(flags)
	if err := flags.Parse(args); err != nil {
		flags.Usage()
		return nil, err
	}

	if err := cmd.Parse(flags.Args()); err != nil {
		flags.Usage()
		return nil, err
	}

	return cmd, nil
}

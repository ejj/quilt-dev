package cli

import (
	"flag"
	"os"

	"github.com/kelda/kelda/cli/command"
	"github.com/kelda/kelda/cli/command/inspect"

	log "github.com/sirupsen/logrus"
)

// Note the `minion` command is in quiltctl_posix.go as it only runs on posix systems.
var commands = map[string]command.SubCommand{
	"daemon":  command.NewDaemonCommand(),
	"inspect": &inspect.Inspect{},
	"logs":    command.NewLogCommand(),

	"ps":   command.NewShowCommand(),
	"show": command.NewShowCommand(),

	"run":        command.NewRunCommand(),
	"init":       &command.Init{},
	"ssh":        command.NewSSHCommand(),
	"stop":       command.NewStopCommand(),
	"version":    command.NewVersionCommand(),
	"debug-logs": command.NewDebugCommand(),
	"counters":   &command.Counters{},
}

// Run parses and runs the cli subcommand given the command line arguments.
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

// HasSubcommand returns true if there is a subcommand for the given name.
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

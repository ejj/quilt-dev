package inspect

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kelda/kelda/blueprint"
	"github.com/kelda/kelda/util"
)

var inspCommands = "quilt inspect BLUEPRINT OUTPUT_FORMAT"
var inspExplanation = `Visualize a blueprint.

OUTPUT_FORMAT can be pdf, ascii, or graphviz.

Dependencies:
 - easy-graph (install Graph::Easy from cpan)
 - graphviz (install from your favorite package manager)`

// Inspect contains the options for inspecting Blueprints.
type Inspect struct {
	configPath string
	outputType string
}

// InstallFlags sets up parsing for command line flags.
func (iCmd *Inspect) InstallFlags(flags *flag.FlagSet) {
	flags.Usage = func() {
		util.PrintUsageString(
			inspCommands, inspExplanation, &flag.FlagSet{})
	}
}

// Parse parses the command line arguments for the inspect command.
func (iCmd *Inspect) Parse(args []string) error {
	if arglen := len(args); arglen < 2 {
		return errors.New("not enough arguments")
	}
	iCmd.configPath = args[0]

	iCmd.outputType = args[1]
	switch iCmd.outputType {
	case "pdf", "ascii", "graphviz":
		return nil
	default:
		return errors.New("output type not supported")
	}
}

// BeforeRun makes any necessary post-parsing transformations.
func (iCmd *Inspect) BeforeRun() error {
	return nil
}

// AfterRun performs any necessary post-run cleanup.
func (iCmd *Inspect) AfterRun() error {
	return nil
}

// Run inspects the provided Blueprint.
func (iCmd *Inspect) Run() int {
	bp, err := blueprint.FromFile(iCmd.configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	graph, err := New(bp)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	viz(iCmd.configPath, graph, iCmd.outputType)

	return 0
}

package inspect

import (
	"flag"
	"fmt"
	"os"

	"github.com/quilt/quilt/stitch"
	"github.com/quilt/quilt/util"
)

var inspCommands = "quilt inspect BLUEPRINT OUTPUT_FORMAT"
var inspExplanation = `Visualize a blueprint.

OUTPUT_FORMAT can be pdf, ascii, or graphviz.

Dependencies:
 - easy-graph (install Graph::Easy from cpan)
 - graphviz (install from your favorite package manager)`

// Usage prints the usage string for the inspect tool.
func Usage() {
	util.PrintUsageString(inspCommands, inspExplanation, &flag.FlagSet{})
}

// Main is the main function for inspect tool. Helps visualize stitches.
func Main(opts []string) int {
	if arglen := len(opts); arglen < 2 {
		fmt.Println("not enough arguments: ", arglen)
		Usage()
		return 1
	}

	configPath := opts[0]

	blueprint, err := stitch.FromFile(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	graph, err := stitch.InitializeGraph(blueprint)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	switch opts[1] {
	case "pdf", "ascii", "graphviz":
		viz(configPath, blueprint, graph, opts[1])
	default:
		Usage()
		return 1
	}

	return 0
}

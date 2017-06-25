package command

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/quilt/quilt/api/pb"
)

var usage = `usage: quilt counters [-H=<daemon_host>]
quilt counters displays internal counters tracked for
debugging purposes.  It's intended for Quilt experts.

`

// Counters implements the `quilt counters` command.
type Counters struct {
	connectionHelper
}

// InstallFlags sets up parsing for command line flags.
func (cmd *Counters) InstallFlags(flags *flag.FlagSet) {
	cmd.connectionHelper.InstallFlags(flags)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		flags.PrintDefaults()
	}
}

// Parse parses the command line arguments for the counters command.
func (cmd *Counters) Parse(args []string) error {
	return nil
}

// Run retrieves and prints all machines and containers.
func (cmd *Counters) Run() int {
	if err := cmd.run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}
	return 0
}

func (cmd *Counters) run() error {
	counters, err := cmd.client.QueryCounters()
	if err != nil {
		return fmt.Errorf("error querying debug counters: %s", err)
	}

	printCounters(os.Stdout, counters)
	return nil
}

func printCounters(out io.Writer, counters []pb.Counter) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "COUNTER\tVALUE\tDELTA\n\t\t\n")

	byPkg := map[string][]pb.Counter{}
	for _, c := range counters {
		byPkg[c.Pkg] = append(byPkg[c.Pkg], c)
	}

	var packages []string
	for p := range byPkg {
		packages = append(packages, p)
	}
	sort.Strings(packages)

	for _, pkg := range packages {
		fmt.Fprintf(w, "%s\t\t\t\n", pkg)

		byName := map[string]pb.Counter{}
		for _, c := range byPkg[pkg] {
			byName[c.Name] = c
		}

		var names []string
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)

		for _, n := range names {
			val := byName[n].Value
			prev := byName[n].PrevValue
			fmt.Fprintf(w, "    %s\t%d\t%d\n", n, val, val-prev)
		}
	}
}

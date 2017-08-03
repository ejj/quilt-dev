package command

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/fatih/color"
	"github.com/pmezard/go-difflib/difflib"

	"github.com/quilt/quilt/api/client"
	"github.com/quilt/quilt/stitch"
	"github.com/quilt/quilt/util"
)

// Run contains the options for running Stitches.
type Run struct {
	stitch string
	force  bool

	connectionHelper
}

// NewRunCommand creates a new Run command instance.
func NewRunCommand() *Run {
	return &Run{}
}

var runCommands = `quilt run [OPTIONS] BLUEPRINT`
var runExplanation = `Compile a blueprint, and deploy the system it describes.

Confirmation is required if deploying the blueprint would change an existing
deployment. Confirmation can be skipped with the -f flag.`

// InstallFlags sets up parsing for command line flags.
func (rCmd *Run) InstallFlags(flags *flag.FlagSet) {
	rCmd.connectionHelper.InstallFlags(flags)

	flags.StringVar(&rCmd.stitch, "blueprint", "", "the blueprint to run")
	flags.BoolVar(&rCmd.force, "f", false, "deploy without confirming changes")

	flags.Usage = func() {
		util.PrintUsageString(runCommands, runExplanation, flags)
	}
}

// Parse parses the command line arguments for the run command.
func (rCmd *Run) Parse(args []string) error {
	if rCmd.stitch == "" {
		if len(args) == 0 {
			return errors.New("no blueprint specified")
		}
		rCmd.stitch = args[0]
	}

	return nil
}

var errNoBlueprint = errors.New("no blueprint")

var compile = stitch.FromFile

// Run starts the run for the provided Stitch.
func (rCmd *Run) Run() int {
	compiled, err := compile(rCmd.stitch)
	if err != nil {
		log.Error(err)
		return 1
	}
	deployment := compiled.String()

	curr, err := getCurrentDeployment(rCmd.client)
	if err != nil && err != errNoBlueprint {
		log.WithError(err).Error("Unable to get current deployment.")
		return 1
	}

	if !rCmd.force && err != errNoBlueprint {
		diff, err := diffDeployment(curr.String(), deployment)
		if err != nil {
			log.WithError(err).Error("Unable to diff deployments.")
			return 1
		}

		if diff == "" {
			fmt.Println("No change.")
		} else {
			fmt.Println(colorizeDiff(diff))
		}
		shouldDeploy, err := confirm(os.Stdin, "Continue with deployment?")
		if err != nil {
			log.WithError(err).Error("Unable to get user response.")
			return 1
		}

		if !shouldDeploy {
			fmt.Println("Deployment aborted by user.")
			return 0
		}
	}

	err = rCmd.client.Deploy(deployment)
	if err != nil {
		log.WithError(err).Error("Error while starting run.")
		return 1
	}

	log.Debug("Successfully started run")
	return 0
}

func getCurrentDeployment(c client.Client) (stitch.Stitch, error) {
	blueprints, err := c.QueryBlueprints()
	if err != nil {
		return stitch.Stitch{}, err
	}
	switch len(blueprints) {
	case 0:
		return stitch.Stitch{}, errNoBlueprint
	case 1:
		return stitch.FromJSON(blueprints[0].Blueprint)
	default:
		panic("unreached")
	}
}

func diffDeployment(currRaw, newRaw string) (string, error) {
	curr, err := prettifyJSON(currRaw)
	if err != nil {
		return "", err
	}
	new, err := prettifyJSON(newRaw)
	if err != nil {
		return "", err
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(curr),
		B:        difflib.SplitLines(new),
		FromFile: "Current",
		ToFile:   "Proposed",
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}

func prettifyJSON(toPrettify string) (string, error) {
	var prettified bytes.Buffer
	err := json.Indent(&prettified, []byte(toPrettify), "", "\t")
	if err != nil {
		return "", err
	}
	return prettified.String(), nil
}

func colorizeDiff(toColorize string) string {
	var colorized bytes.Buffer
	for _, line := range strings.SplitAfter(toColorize, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			colorized.WriteString(color.GreenString("%s", line))
		case strings.HasPrefix(line, "-"):
			colorized.WriteString(color.RedString("%s", line))
		default:
			colorized.WriteString(line)
		}
	}
	return colorized.String()
}

// Saved in a variable so that we can mock it for unit testing.
var confirm = func(in io.Reader, prompt string) (bool, error) {
	reader := bufio.NewReader(in)

	for {
		fmt.Printf("%s [y/n]: ", prompt)

		response, _, err := reader.ReadLine()
		if err != nil {
			return false, err
		}

		switch strings.ToLower(strings.TrimSpace(string(response))) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
	}
}

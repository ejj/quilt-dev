package command

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	units "github.com/docker/go-units"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/stitch"
	"github.com/quilt/quilt/util"
)

// An arbitrary length to truncate container commands to.
const truncLength = 30

// Ps contains the options for querying machines and containers.
type Ps struct {
	noTruncate bool

	connectionHelper
}

// NewPsCommand creates a new Ps command instance.
func NewPsCommand() *Ps {
	return &Ps{}
}

var psCommands = "quilt ps [-H=<daemon_host>]"
var psExplanation = "Display the status of quilt-managed machines and containers."

// InstallFlags sets up parsing for command line flags
func (pCmd *Ps) InstallFlags(flags *flag.FlagSet) {
	pCmd.connectionHelper.InstallFlags(flags)
	flags.BoolVar(&pCmd.noTruncate, "no-trunc", false, "do not truncate container"+
		" command output")
	flags.Usage = func() {
		util.PrintUsageString(psCommands, psExplanation, flags)
	}
}

// Parse parses the command line arguments for the ps command.
func (pCmd *Ps) Parse(args []string) error {
	return nil
}

// Run retrieves and prints all machines and containers.
func (pCmd *Ps) Run() int {
	if err := pCmd.run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}
	return 0
}

func (pCmd *Ps) run() (err error) {
	machines, err := pCmd.client.QueryMachines()
	if err != nil {
		return fmt.Errorf("unable to query machines: %s", err)
	}

	writeMachines(os.Stdout, machines)
	fmt.Println()

	clusterUp := false
	for _, m := range machines {
		if m.Status == db.Connected || m.Status == db.Reconnecting {
			clusterUp = true
		}
	}

	// Only attempt to query container information if the foreman has connected
	// to a machine. If the foreman hasn't connected to any machines, then there's
	// no way any containers could be running because the deployment hasn't been
	// sent to the cluster yet.
	if !clusterUp {
		return nil
	}

	var connections []db.Connection
	var containers []db.Container
	var images []db.Image
	connectionErr := make(chan error)
	containerErr := make(chan error)
	imagesErr := make(chan error)

	go func() {
		connections, err = pCmd.client.QueryConnections()
		connectionErr <- err
	}()

	go func() {
		containers, err = pCmd.client.QueryContainers()
		containerErr <- err
	}()

	go func() {
		images, err = pCmd.client.QueryImages()
		imagesErr <- err
	}()

	if err := <-connectionErr; err != nil {
		return fmt.Errorf("unable to query connections: %s", err)
	}
	if err := <-containerErr; err != nil {
		return fmt.Errorf("unable to query containers: %s", err)
	}
	if err := <-imagesErr; err != nil {
		return fmt.Errorf("unable to query images: %s", err)
	}

	writeContainers(os.Stdout, containers, machines, connections, images,
		!pCmd.noTruncate)

	return nil
}

func writeMachines(fd io.Writer, machines []db.Machine) {
	w := tabwriter.NewWriter(fd, 0, 0, 4, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "MACHINE\tROLE\tPROVIDER\tREGION\tSIZE\tPUBLIC IP\tSTATUS")

	for _, m := range db.SortMachines(machines) {
		// Prefer the floating IP over the public IP if it's defined.
		pubIP := m.PublicIP
		if m.FloatingIP != "" {
			pubIP = m.FloatingIP
		}

		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			util.ShortUUID(m.StitchID), m.Role, m.Provider, m.Region, m.Size,
			pubIP, m.Status)
	}
}

func writeContainers(fd io.Writer, containers []db.Container, machines []db.Machine,
	connections []db.Connection, images []db.Image, truncate bool) {
	w := tabwriter.NewWriter(fd, 0, 0, 4, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "CONTAINER\tMACHINE\tCOMMAND\tLABELS"+
		"\tSTATUS\tCREATED\tPUBLIC IP")

	labelPublicPorts := map[string][]string{}
	for _, c := range connections {
		if c.From != stitch.PublicInternetLabel {
			continue
		}

		portStr := fmt.Sprintf("%d", c.MinPort)
		if c.MinPort != c.MaxPort {
			portStr += fmt.Sprintf("-%d", c.MaxPort)
		}
		labelPublicPorts[c.To] = append(labelPublicPorts[c.To], portStr)
	}

	ipIDMap := map[string]string{}
	idMachineMap := map[string]db.Machine{}
	for _, m := range machines {
		ipIDMap[m.PrivateIP] = m.StitchID
		idMachineMap[m.StitchID] = m
	}

	machineDBC := map[string][]db.Container{}
	for _, dbc := range containers {
		id := ipIDMap[dbc.Minion]
		machineDBC[id] = append(machineDBC[id], dbc)
	}

	var machineIDs []string
	for key := range machineDBC {
		machineIDs = append(machineIDs, key)
	}
	sort.Strings(machineIDs)

	imageStatusMap := map[string]string{}
	for _, img := range images {
		imageStatusMap[img.Name] = img.Status
	}

	for i, machineID := range machineIDs {
		if i > 0 {
			// Insert a blank line between each machine.
			// Need to print tabs in a blank line; otherwise, spacing will
			// change in subsequent lines.
			fmt.Fprintf(w, "\t\t\t\t\t\t\n")
		}

		dbcs := machineDBC[machineID]
		sort.Sort(db.ContainerSlice(dbcs))
		for _, dbc := range dbcs {
			publicPorts := []string{}
			for _, label := range dbc.Labels {
				if p, ok := labelPublicPorts[label]; ok {
					publicPorts = append(publicPorts, p...)
				}
			}

			container := containerStr(dbc.Image, dbc.Command, truncate)
			labels := strings.Join(dbc.Labels, ", ")

			var status string
			switch {
			case dbc.Status != "":
				status = dbc.Status
			case dbc.Minion != "":
				status = "scheduled"
			default:
				if imgStatus, ok := imageStatusMap[dbc.Image]; ok {
					status = imgStatus
				}
			}

			created := ""
			if !dbc.Created.IsZero() {
				createdTime := dbc.Created.Local()
				duration := units.HumanDuration(time.Since(createdTime))
				created = fmt.Sprintf("%s ago", duration)
			}

			publicIP := publicIPStr(idMachineMap[machineID], publicPorts)

			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				util.ShortUUID(dbc.StitchID), util.ShortUUID(machineID),
				container, labels, status, created, publicIP)
		}
	}
}

func containerStr(image string, args []string, truncate bool) string {
	if image == "" {
		return ""
	}

	container := fmt.Sprintf("%s %s", image, strings.Join(args, " "))
	if truncate && len(container) > truncLength {
		return container[:truncLength] + "..."
	}

	return container
}

func publicIPStr(m db.Machine, publicPorts []string) string {
	// Prefer the floating IP over the public IP if it's defined.
	hostPublicIP := m.PublicIP
	if m.FloatingIP != "" {
		hostPublicIP = m.FloatingIP
	}

	if hostPublicIP == "" || len(publicPorts) == 0 {
		return ""
	}

	if len(publicPorts) == 1 {
		return fmt.Sprintf("%s:%s", hostPublicIP, publicPorts[0])
	}

	return fmt.Sprintf("%s:[%s]", hostPublicIP, strings.Join(publicPorts, ","))
}

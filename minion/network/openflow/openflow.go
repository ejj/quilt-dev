package openflow

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/quilt/quilt/counter"
	"github.com/quilt/quilt/minion/ipdef"
	"github.com/quilt/quilt/minion/ovsdb"
)

/* OpenFlow Psuedocode -- Please, for the love of God, keep this updated.

OpenFlow is extremely difficult to reason about -- especially when its buried in Go code.
This comment aims to make it a bit easier to maintain by describing abstractly what the
OpenFlow code does, without the distraction of the go code required to implement it.

Interpreting the Psuedocode
---------------------------
The OpenFlow code is divided into a series of tables.  Packets start at Table_0 and only
move to another table if explicitly instructed to by a `goto` statement.

Each table is composed of a series of if statements.  Packets match either one or zero of
these statements.  If they match zero they're dropped, if they match more than one then
the statement that appears first in the table is chosen.

Each if statement has one or more actions associated with it.  Packets matching the
statement execute those actions in order.  If one of those actions is a goto statement,
the packet is forwarded to the specified table and the process begins again.

Finally, note that some tables have loops which should be interpreted as duplicating the
inner if statements per loop element.

Registers
---------

The psuedocode currently uses three registers:

Reg0 -- Contains the OpenFlow port number of the patch port if the packet came from a
veth. Otherwise it contains zero.

Tables
------

// Table_0 initializes the registers and forwards to Table_1.
Table_0 { // Initial Table
	for each db.Container {
		if in_port=dbc.VethPort && dl_src=dbc.Mac {
			reg0 <- dbc.PatchPort
			goto Table_1
		}

		if in_port=dbc.PatchPort {
			output:dbc.VethPort
		}
	}

	if in_port=LOCAL {
		goto Table_2
	}
}

// Table_1 handles packets coming from a veth.
Table_1 {
	// Send broadcasts to the gateway and patch port.
	if dl_dst=ff:ff:ff:ff:ff:ff {
		output:LOCAL,reg0
	}

	// Send packets from the veth to the gateway.
	if dl_dst=gwMac {
		output:LOCAL
	}

	// Everything else can be handled by OVN.
	output:reg0
}

// Table_2 forwards packets coming from the LOCAL port.
Table_2 {
	// If the gateway sends a broadcast, send it to all veths.
	if dl_dst=ff:ff:ff:ff:ff:ff {
		output:veth{1..n}
	}

	// Otherwise output to the veth based on the dest mac.
	for each db.Container {
		if dl_dst=dbc.Mac {
			output:veth
		}
	}
}
*/

// A Container that needs OpenFlow rules installed for it.
type Container struct {
	Veth  string
	Patch string
	Mac   string
}

type container struct {
	Container

	vethPort  int
	patchPort int
}

var c = counter.New("OpenFlow")

var staticFlows = []string{
	// Table 0
	"table=0,priority=1000,in_port=LOCAL,actions=resubmit(,2)",

	// Table 1
	"table=1,priority=1000,dl_dst=ff:ff:ff:ff:ff:ff," +
		"actions=output:LOCAL,output:NXM_NX_REG0[]",
	fmt.Sprintf("table=1,priority=900,dl_dst=%s,actions=LOCAL", ipdef.GatewayMac),
	"table=1,priority=800,actions=output:NXM_NX_REG0[]"}

// ReplaceFlows adds flows associated with the provided containers, and removes all
// other flows.
func ReplaceFlows(containers []Container) error {
	c.Inc("Replace Flows")
	ofports, err := openflowPorts()
	if err != nil {
		return err
	}

	flows := allFlows(resolveContainers(ofports, containers))
	// XXX: Due to a bug in `ovs-ofctl replace-flows`, certain flows are
	// replaced even if they do not differ. `diff-flows` already has a fix to
	// this problem, so for now we only run `replace-flows` when `diff-flows`
	// reports no changes.  The `diff-flows` check should be removed once
	// `replace-flows` is fixed upstream.
	if ofctl("diff-flows", flows) != nil {
		c.Inc("Flows Changed")
		if err := ofctl("replace-flows", flows); err != nil {
			return fmt.Errorf("ovs-ofctl: %s", err)
		}
	}

	return nil
}

// AddFlows adds flows associated with the provided containers without touching flows
// that may already be installed.
func AddFlows(containers []Container) error {
	c.Inc("Add Flows")
	ofports, err := openflowPorts()
	if err != nil {
		return err
	}

	flows := allContainerFlows(resolveContainers(ofports, containers))
	if err := ofctl("add-flows", flows); err != nil {
		return fmt.Errorf("ovs-ofctl: %s", err)
	}

	return nil
}

func allContainerFlows(containers []container) []string {
	var flows []string
	for _, c := range containers {
		flows = append(flows, containerFlows(c)...)
	}
	return flows
}

func containerFlows(c container) []string {
	flows := []string{
		// Table 0
		fmt.Sprintf("table=0,in_port=%d,dl_src=%s,"+
			"actions=load:0x%x->NXM_NX_REG0[],resubmit(,1)",
			c.vethPort, c.Mac, c.patchPort),
		fmt.Sprintf("table=0,in_port=%d,actions=output:%d",
			c.patchPort, c.vethPort),

		// Table 2
		fmt.Sprintf("table=2,priority=900,dl_dst=%s,action=output:%d",
			c.Mac, c.vethPort),
	}

	return flows
}

func allFlows(containers []container) []string {
	var gatewayBroadcastActions []string
	for _, c := range containers {
		gatewayBroadcastActions = append(gatewayBroadcastActions,
			fmt.Sprintf("output:%d", c.vethPort))
	}

	flows := append(staticFlows, allContainerFlows(containers)...)
	return append(flows, "table=2,priority=1000,dl_dst=ff:ff:ff:ff:ff:ff,actions="+
		strings.Join(gatewayBroadcastActions, ","))
}

func resolveContainers(portMap map[string]int, containers []Container) []container {
	var ofcs []container
	for _, c := range containers {
		veth, okVeth := portMap[c.Veth]
		patch, okPatch := portMap[c.Patch]
		if !okVeth || !okPatch {
			continue
		}

		ofcs = append(ofcs, container{
			Container: c,
			patchPort: patch,
			vethPort:  veth,
		})
	}
	return ofcs
}

func openflowPorts() (map[string]int, error) {
	odb, err := ovsdb.Open()
	if err != nil {
		return nil, fmt.Errorf("ovsdb-server connection: %s", err)
	}
	defer odb.Disconnect()

	return odb.OpenFlowPorts()
}

var ofctl = func(action string, flows []string) error {
	c.Inc("ovs-ofctl")
	cmd := exec.Command("ovs-ofctl", "-O", "OpenFlow13", action,
		ipdef.QuiltBridge, "/dev/stdin")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	for _, f := range flows {
		stdin.Write([]byte(f + "\n"))
	}
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

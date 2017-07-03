package google

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/quilt/quilt/cluster/acl"
	"github.com/quilt/quilt/cluster/cloudcfg"
	"github.com/quilt/quilt/cluster/google/client"
	"github.com/quilt/quilt/cluster/machine"
	"github.com/quilt/quilt/join"

	log "github.com/Sirupsen/logrus"
	"github.com/satori/go.uuid"
	compute "google.golang.org/api/compute/v1"
)

// DefaultRegion is the preferred location for machines that don't have a
// user specified region preference.
const DefaultRegion = "us-east1-b"

// Zones is the list of supported GCE zones
var Zones = []string{"us-central1-a", "us-east1-b", "europe-west1-b"}

// ephemeralIPName is a constant for what we label NATs with ephemeral IPs in GCE.
const ephemeralIPName = "External NAT"

// floatingIPName is a constant for what we label NATs with floating IPs in GCE.
const floatingIPName = "Floating IP"

const computeBaseURL string = "https://www.googleapis.com/compute/v1/projects"
const (
	// These are the various types of Operations that the GCE API returns
	local = iota
	global
)

// The Cluster objects represents a connection to GCE.
type Cluster struct {
	gce client.Client

	imgURL      string // gce url to the VM image
	networkName string // gce identifier for the network
	ipv4Range   string // ipv4 range of the internal network
	intFW       string // gce internal firewall name
	zone        string // gce boot region

	ns string // cluster namespace
}

// New creates a GCE cluster.
//
// Clusters are differentiated (namespace) by setting the description and
// filtering off of that.
func New(namespace, zone string) (*Cluster, error) {
	gce, err := client.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCE client: %s", err.Error())
	}

	clst := Cluster{
		gce:       gce,
		ns:        namespace,
		ipv4Range: "192.168.0.0/16",
		zone:      zone,
	}
	clst.intFW = fmt.Sprintf("%s-internal", clst.ns)
	clst.imgURL = fmt.Sprintf("%s/%s", computeBaseURL,
		"ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20170202")
	clst.networkName = clst.ns

	if err := clst.createNetwork(); err != nil {
		log.WithError(err).Debug("failed to start up gce network")
		return nil, err
	}

	return &clst, nil
}

// List the current machines in the cluster.
func (clst *Cluster) List() ([]machine.Machine, error) {
	var mList []machine.Machine
	list, err := clst.gce.ListInstances(clst.zone,
		fmt.Sprintf("description eq %s", clst.ns))
	if err != nil {
		return nil, err
	}
	for _, item := range list.Items {
		// XXX: This make some iffy assumptions about NetworkInterfaces
		machineSplitURL := strings.Split(item.MachineType, "/")
		mtype := machineSplitURL[len(machineSplitURL)-1]

		accessConfig := item.NetworkInterfaces[0].AccessConfigs[0]
		floatingIP := ""
		if accessConfig.Name == floatingIPName {
			floatingIP = accessConfig.NatIP
		}

		mList = append(mList, machine.Machine{
			ID:         item.Name,
			PublicIP:   accessConfig.NatIP,
			FloatingIP: floatingIP,
			PrivateIP:  item.NetworkInterfaces[0].NetworkIP,
			Size:       mtype,
		})
	}
	return mList, nil
}

// Boot blocks while creating instances.
func (clst *Cluster) Boot(bootSet []machine.Machine) error {
	// XXX: should probably have a better clean up routine if an error is encountered
	var names []string
	for _, m := range bootSet {
		if m.Preemptible {
			return errors.New("preemptible instances are not yet implemented")
		}

		name := "quilt-" + uuid.NewV4().String()
		_, err := clst.instanceNew(name, m.Size,
			cloudcfg.Ubuntu(m.CloudCfgOpts))
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"id":    m.ID,
			}).Error("Failed to start instance.")
			continue
		}
		names = append(names, name)
	}
	if err := clst.wait(names, true); err != nil {
		return err
	}
	return nil
}

// Stop blocks while deleting the instances.
//
// If an error occurs while deleting, it will finish the ones that have
// successfully started before returning.
func (clst *Cluster) Stop(machines []machine.Machine) error {
	// XXX: should probably have a better clean up routine if an error is encountered
	var names []string
	for _, m := range machines {
		_, err := clst.gce.DeleteInstance(clst.zone, m.ID)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"id":    m.ID,
			}).Error("Failed to delete instance.")
			continue
		}
		names = append(names, m.ID)
	}
	if err := clst.wait(names, false); err != nil {
		return err
	}
	return nil
}

// Get() and operationWait() don't always present the same results, so
// Boot() and Stop() must have a special wait to stay in sync with Get().
func (clst *Cluster) wait(names []string, live bool) error {
	if len(names) == 0 {
		return nil
	}

	after := time.After(3 * time.Minute)
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()

	for range tick.C {
		select {
		case <-after:
			return errors.New("wait(): timeout")
		default:
		}

		for len(names) > 0 {
			name := names[0]
			instances, err := clst.List()
			if err != nil {
				return err
			}
			exists := false
			for _, ist := range instances {
				if name == ist.ID {
					exists = true
				}
			}
			if live == exists {
				names = append(names[:0], names[1:]...)
			}
		}
		if len(names) == 0 {
			return nil
		}
	}
	return nil
}

// Blocking wait with a hardcoded timeout.
//
// Waits on operations, the type of which is indicated by 'domain'. All
// operations must be of the same 'domain'
func (clst *Cluster) operationWait(ops []*compute.Operation, domain int) error {
	if len(ops) == 0 {
		return nil
	}

	after := time.After(3 * time.Minute)
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()

	var op *compute.Operation
	var err error
	for {
		select {
		case <-after:
			return fmt.Errorf("operationWait(): timeout")
		case <-tick.C:
			for len(ops) > 0 {
				switch {
				case domain == local:
					op, err = clst.gce.GetZoneOperation(
						ops[0].Zone, ops[0].Name)
				case domain == global:
					op, err = clst.gce.GetGlobalOperation(ops[0].Name)
				}
				if err != nil {
					return err
				}
				if op.Status != "DONE" {
					break
				}
				ops = append(ops[:0], ops[1:]...)
			}
			if len(ops) == 0 {
				return nil
			}
		}
	}
}

// Create new GCE instance.
//
// Does not check if the operation succeeds.
func (clst *Cluster) instanceNew(name string, size string,
	cloudConfig string) (*compute.Operation, error) {
	instance := &compute.Instance{
		Name:        name,
		Description: clst.ns,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s",
			clst.zone,
			size),
		Disks: []*compute.AttachedDisk{
			{
				Boot:       true,
				AutoDelete: true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: clst.imgURL,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: ephemeralIPName,
					},
				},
				Network: networkURL(clst.networkName),
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "startup-script",
					Value: &cloudConfig,
				},
			},
		},
		Tags: &compute.Tags{
			// Tag the machine with its zone so that we can create zone-scoped
			// firewall rules.
			Items: []string{clst.zone},
		},
	}

	return clst.gce.InsertInstance(clst.zone, instance)
}

// listFirewalls returns the firewalls managed by the cluster. Specifically,
// it returns all firewalls that are attached to the cluster's network, and
// apply to the managed zone.
func (clst Cluster) listFirewalls() ([]compute.Firewall, error) {
	firewalls, err := clst.gce.ListFirewalls()
	if err != nil {
		return nil, fmt.Errorf("list firewalls: %s", err)
	}

	var fws []compute.Firewall
	for _, fw := range firewalls.Items {
		_, nwName := path.Split(fw.Network)
		if nwName != clst.networkName || fw.Name == clst.intFW {
			continue
		}

		for _, tag := range fw.TargetTags {
			if tag == clst.zone {
				fws = append(fws, *fw)
				break
			}
		}
	}

	return fws, nil
}

// parseACLs parses the firewall rules contained in the given firewall into
// `acl.ACL`s.
// parseACLs only handles rules specified in the format that Quilt generates: it
// does not handle all the possible rule strings supported by the Google API.
func (clst *Cluster) parseACLs(fws []compute.Firewall) (acls []acl.ACL, err error) {
	for _, fw := range fws {
		portACLs, err := parsePorts(fw.Allowed)
		if err != nil {
			return nil, fmt.Errorf("parse ports of %s: %s", fw.Name, err)
		}

		for _, cidrIP := range fw.SourceRanges {
			for _, acl := range portACLs {
				acl.CidrIP = cidrIP
				acls = append(acls, acl)
			}
		}
	}

	return acls, nil
}

func parsePorts(allowed []*compute.FirewallAllowed) (acls []acl.ACL, err error) {
	for _, rule := range allowed {
		for _, portsStr := range rule.Ports {
			portRange, err := parseInts(strings.Split(portsStr, "-"))
			if err != nil {
				return nil, fmt.Errorf("parse ints: %s", err)
			}

			var min, max int
			switch len(portRange) {
			case 1:
				min, max = portRange[0], portRange[0]
			case 2:
				min, max = portRange[0], portRange[1]
			default:
				return nil, fmt.Errorf(
					"unrecognized port format: %s", portsStr)
			}
			acls = append(acls, acl.ACL{MinPort: min, MaxPort: max})
		}
	}
	return acls, nil
}

func parseInts(intStrings []string) (parsed []int, err error) {
	for _, str := range intStrings {
		parsedInt, err := strconv.Atoi(str)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, parsedInt)
	}
	return parsed, nil
}

// SetACLs adds and removes acls in `clst` so that it conforms to `acls`.
func (clst *Cluster) SetACLs(acls []acl.ACL) error {
	fws, err := clst.listFirewalls()
	if err != nil {
		return err
	}

	currACLs, err := clst.parseACLs(fws)
	if err != nil {
		return fmt.Errorf("parse ACLs: %s", err)
	}

	pair, toAdd, toRemove := join.HashJoin(acl.Slice(acls), acl.Slice(currACLs),
		nil, nil)

	var toSet []acl.ACL
	for _, a := range toAdd {
		toSet = append(toSet, a.(acl.ACL))
	}
	for _, p := range pair {
		toSet = append(toSet, p.L.(acl.ACL))
	}
	for _, a := range toRemove {
		toSet = append(toSet, acl.ACL{
			MinPort: a.(acl.ACL).MinPort,
			MaxPort: a.(acl.ACL).MaxPort,
			CidrIP:  "", // Remove all currently allowed IPs.
		})
	}

	for acl, cidrIPs := range groupACLsByPorts(toSet) {
		fw, err := clst.getCreateFirewall(acl.MinPort, acl.MaxPort)
		if err != nil {
			return err
		}

		if reflect.DeepEqual(fw.SourceRanges, cidrIPs) {
			continue
		}

		var op *compute.Operation
		if len(cidrIPs) == 0 {
			log.WithField("ports", fmt.Sprintf(
				"%d-%d", acl.MinPort, acl.MaxPort)).
				Debug("Google: Deleting firewall")
			op, err = clst.gce.DeleteFirewall(fw.Name)
			if err != nil {
				return err
			}
		} else {
			log.WithField("ports", fmt.Sprintf(
				"%d-%d", acl.MinPort, acl.MaxPort)).
				WithField("CidrIPs", cidrIPs).
				Debug("Google: Setting ACLs")
			op, err = clst.firewallPatch(fw.Name, cidrIPs)
			if err != nil {
				return err
			}
		}
		if err := clst.operationWait(
			[]*compute.Operation{op}, global); err != nil {
			return err
		}
	}

	return nil
}

// UpdateFloatingIPs updates IPs of machines by recreating their network interfaces.
func (clst *Cluster) UpdateFloatingIPs(machines []machine.Machine) error {
	for _, m := range machines {
		instance, err := clst.gce.GetInstance(clst.zone, m.ID)
		if err != nil {
			return err
		}

		// Delete existing network interface. It is only possible to assign
		// one access config per instance. Thus, updating GCE Floating IPs
		// is not a seamless, zero-downtime procedure.
		networkInterface := instance.NetworkInterfaces[0]
		accessConfig := instance.NetworkInterfaces[0].AccessConfigs[0]
		_, err = clst.gce.DeleteAccessConfig(clst.zone, m.ID,
			accessConfig.Name, networkInterface.Name)
		if err != nil {
			return err
		}

		// Add new network interface.
		_, err = clst.gce.AddAccessConfig(clst.zone, m.ID,
			networkInterface.Name, &compute.AccessConfig{
				Type: "ONE_TO_ONE_NAT",
				Name: floatingIPName,
				// Google will automatically assign a dynamic IP
				// if none is provided (i.e. m.FloatingIP == "").
				NatIP: m.FloatingIP,
			})
		if err != nil {
			return err
		}
	}

	return nil
}

func (clst *Cluster) getFirewall(name string) (*compute.Firewall, error) {
	list, err := clst.gce.ListFirewalls()
	if err != nil {
		return nil, err
	}
	for _, val := range list.Items {
		if val.Name == name {
			return val, nil
		}
	}

	return nil, nil
}

func (clst *Cluster) getCreateFirewall(minPort int, maxPort int) (
	*compute.Firewall, error) {

	ports := fmt.Sprintf("%d-%d", minPort, maxPort)
	fwName := fmt.Sprintf("%s-%s-%s", clst.ns, clst.zone, ports)

	if fw, _ := clst.getFirewall(fwName); fw != nil {
		return fw, nil
	}

	log.WithField("name", fwName).Debug("Creating firewall")
	op, err := clst.insertFirewall(fwName, ports, []string{"127.0.0.1/32"}, true)
	if err != nil {
		return nil, err
	}

	if err := clst.operationWait([]*compute.Operation{op}, global); err != nil {
		return nil, err
	}

	return clst.getFirewall(fwName)
}

func (clst *Cluster) networkExists(name string) (bool, error) {
	list, err := clst.gce.ListNetworks()
	if err != nil {
		return false, err
	}
	for _, val := range list.Items {
		if val.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// This creates a firewall but does nothing else
func (clst *Cluster) insertFirewall(name, ports string, sourceRanges []string,
	restrictToZone bool) (*compute.Operation, error) {

	var targetTags []string
	if restrictToZone {
		targetTags = []string{clst.zone}
	}

	firewall := &compute.Firewall{
		Name:    name,
		Network: networkURL(clst.networkName),
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{ports},
			},
			{
				IPProtocol: "udp",
				Ports:      []string{ports},
			},
			{
				IPProtocol: "icmp",
			},
		},
		SourceRanges: sourceRanges,
		TargetTags:   targetTags,
	}

	return clst.gce.InsertFirewall(firewall)
}

func (clst *Cluster) firewallExists(name string) (bool, error) {
	fw, err := clst.getFirewall(name)
	return fw != nil, err
}

// Updates the firewall using PATCH semantics.
//
// The IP addresses must be in CIDR notation.
func (clst *Cluster) firewallPatch(name string,
	ips []string) (*compute.Operation, error) {
	firewall := &compute.Firewall{
		Name:         name,
		Network:      networkURL(clst.networkName),
		SourceRanges: ips,
	}

	return clst.gce.PatchFirewall(name, firewall)
}

// Initializes the network for the cluster
func (clst *Cluster) createNetwork() error {
	exists, err := clst.networkExists(clst.networkName)
	if err != nil {
		return err
	}

	if exists {
		log.Debug("Network already exists")
		return nil
	}

	log.Debug("Creating network")
	op, err := clst.gce.InsertNetwork(&compute.Network{
		Name:      clst.networkName,
		IPv4Range: clst.ipv4Range,
	})
	if err != nil {
		return err
	}

	err = clst.operationWait([]*compute.Operation{op}, global)
	if err != nil {
		return err
	}
	return clst.createInternalFirewall()
}

// Initializes the internal firewall for the cluster to allow machines to talk
// on the private network.
func (clst *Cluster) createInternalFirewall() error {
	var ops []*compute.Operation

	if exists, err := clst.firewallExists(clst.intFW); err != nil {
		return err
	} else if exists {
		log.Debug("internal firewall already exists")
	} else {
		log.Debug("creating internal firewall")
		op, err := clst.insertFirewall(
			clst.intFW, "1-65535", []string{clst.ipv4Range}, false)
		if err != nil {
			return err
		}
		ops = append(ops, op)
	}

	if err := clst.operationWait(ops, global); err != nil {
		return err
	}
	return nil
}

func networkURL(networkName string) string {
	return fmt.Sprintf("global/networks/%s", networkName)
}

func groupACLsByPorts(acls []acl.ACL) map[acl.ACL][]string {
	grouped := make(map[acl.ACL][]string)
	for _, a := range acls {
		key := acl.ACL{
			MinPort: a.MinPort,
			MaxPort: a.MaxPort,
		}
		if _, ok := grouped[key]; !ok {
			grouped[key] = nil
		}
		if a.CidrIP != "" {
			grouped[key] = append(grouped[key], a.CidrIP)
		}
	}
	return grouped
}

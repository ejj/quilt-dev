package digitalocean

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"

	"github.com/quilt/quilt/cluster/acl"
	"github.com/quilt/quilt/cluster/cloudcfg"
	"github.com/quilt/quilt/cluster/digitalocean/client"
	"github.com/quilt/quilt/cluster/machine"
	"github.com/quilt/quilt/counter"
	"github.com/quilt/quilt/join"
	"github.com/quilt/quilt/util"

	"golang.org/x/oauth2"

	log "github.com/Sirupsen/logrus"
)

// DefaultRegion is assigned to Machines without a specified region
const DefaultRegion string = "sfo1"

// Regions supported by the Digital Ocean API
var Regions = []string{"nyc1", "nyc2", "lon1", "sfo1", "sfo2"}

var c = counter.New("Digital Ocean")

var apiKeyPath = ".digitalocean/key"

var image = "ubuntu-16-04-x64"

// The Cluster object represents a connection to DigitalOcean.
type Cluster struct {
	client    client.Client
	namespace string
	region    string
}

// New starts a new client session with the API key provided in ~/.digitalocean/key.
func New(namespace, region string) (*Cluster, error) {
	clst, err := newDigitalOcean(namespace, region)
	if err != nil {
		return clst, err
	}

	_, _, err = clst.client.ListDroplets(&godo.ListOptions{})
	return clst, err
}

// Creation is broken out for unit testing.
var newDigitalOcean = func(namespace, region string) (*Cluster, error) {
	namespace = strings.ToLower(strings.Replace(namespace, "_", "-", -1))
	keyFile := filepath.Join(os.Getenv("HOME"), apiKeyPath)
	key, err := util.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)

	tc := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: key})
	oauthClient := oauth2.NewClient(oauth2.NoContext, tc)

	clst := &Cluster{
		namespace: namespace,
		region:    region,
		client:    client.New(oauthClient),
	}
	return clst, nil
}

// List will fetch all droplets that have the same name as the cluster namespace.
func (clst Cluster) List() (machines []machine.Machine, err error) {
	floatingIPListOpt := &godo.ListOptions{}
	floatingIPs := map[int]string{}
	for {
		ips, resp, err := clst.client.ListFloatingIPs(floatingIPListOpt)
		if err != nil {
			return nil, fmt.Errorf("list floating IPs: %s", err)
		}

		for _, ip := range ips {
			if ip.Droplet == nil {
				continue
			}
			floatingIPs[ip.Droplet.ID] = ip.IP
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		floatingIPListOpt.Page++
	}

	dropletListOpt := &godo.ListOptions{} // Keep track of the page we're on.
	// DigitalOcean's API has a paginated list of droplets.
	for {
		droplets, resp, err := clst.client.ListDroplets(dropletListOpt)
		if err != nil {
			return nil, fmt.Errorf("list droplets: %s", err)
		}

		for _, d := range droplets {
			if d.Name != clst.namespace || d.Region.Slug != clst.region {
				continue
			}

			pubIP, err := d.PublicIPv4()
			if err != nil {
				return nil, fmt.Errorf("get public IP: %s", err)
			}

			privIP, err := d.PrivateIPv4()
			if err != nil {
				return nil, fmt.Errorf("get private IP: %s", err)
			}

			machine := machine.Machine{
				ID:          strconv.Itoa(d.ID),
				PublicIP:    pubIP,
				PrivateIP:   privIP,
				FloatingIP:  floatingIPs[d.ID],
				Size:        d.SizeSlug,
				Preemptible: false,
			}
			machines = append(machines, machine)
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		dropletListOpt.Page++
	}
	return machines, nil
}

// Boot will boot every machine in a goroutine, and wait for the machines to come up.
func (clst Cluster) Boot(bootSet []machine.Machine) error {
	errChan := make(chan error, len(bootSet))
	for _, m := range bootSet {
		go func(m machine.Machine) {
			errChan <- clst.createAndAttach(m)
		}(m)
	}

	var err error
	for range bootSet {
		if e := <-errChan; e != nil {
			err = e
		}
	}
	return err
}

// Creates a new machine, and waits for the machine to become active.
func (clst Cluster) createAndAttach(m machine.Machine) error {
	cloudConfig := cloudcfg.Ubuntu(m.CloudCfgOpts)
	createReq := &godo.DropletCreateRequest{
		Name:              clst.namespace,
		Region:            clst.region,
		Size:              m.Size,
		Image:             godo.DropletCreateImage{Slug: image},
		PrivateNetworking: true,
		UserData:          cloudConfig,
	}

	d, _, err := clst.client.CreateDroplet(createReq)
	if err != nil {
		return err
	}

	pred := func() bool {
		d, _, err := clst.client.GetDroplet(d.ID)
		return err == nil && d.Status == "active"
	}
	return util.WaitFor(pred, 10*time.Second, 2*time.Minute)
}

// UpdateFloatingIPs updates Droplet to Floating IP associations.
func (clst Cluster) UpdateFloatingIPs(desired []machine.Machine) error {
	curr, err := clst.List()
	if err != nil {
		return fmt.Errorf("list machines: %s", err)
	}

	return clst.syncFloatingIPs(curr, desired)
}

func (clst Cluster) syncFloatingIPs(curr, targets []machine.Machine) error {
	idKey := func(intf interface{}) interface{} {
		return intf.(machine.Machine).ID
	}
	pairs, _, unmatchedDesired := join.HashJoin(
		machine.Slice(curr), machine.Slice(targets), idKey, idKey)

	if len(unmatchedDesired) != 0 {
		var unmatchedIDs []string
		for _, m := range unmatchedDesired {
			unmatchedIDs = append(unmatchedIDs, m.(machine.Machine).ID)
		}
		return fmt.Errorf("no matching IDs: %s", strings.Join(unmatchedIDs, ", "))
	}

	for _, pair := range pairs {
		curr := pair.L.(machine.Machine)
		desired := pair.R.(machine.Machine)

		if curr.FloatingIP == desired.FloatingIP {
			continue
		}

		if curr.FloatingIP != "" {
			_, _, err := clst.client.UnassignFloatingIP(curr.FloatingIP)
			if err != nil {
				return fmt.Errorf("unassign IP (%s): %s",
					curr.FloatingIP, err)
			}
		}

		if desired.FloatingIP != "" {
			id, err := strconv.Atoi(curr.ID)
			if err != nil {
				return fmt.Errorf("malformed id (%s): %s", curr.ID, err)
			}

			_, _, err = clst.client.AssignFloatingIP(desired.FloatingIP, id)
			if err != nil {
				return fmt.Errorf("assign IP (%s to %d): %s",
					desired.FloatingIP, id, err)
			}
		}
	}

	return nil
}

// Stop stops each machine and deletes their attached volumes.
func (clst Cluster) Stop(machines []machine.Machine) error {
	errChan := make(chan error, len(machines))
	for _, m := range machines {
		go func(m machine.Machine) {
			errChan <- clst.deleteAndWait(m.ID)
		}(m)
	}

	var err error
	for range machines {
		if e := <-errChan; e != nil {
			err = e
		}
	}
	return err
}

func (clst Cluster) deleteAndWait(ids string) error {
	id, err := strconv.Atoi(ids)
	if err != nil {
		return err
	}

	_, err = clst.client.DeleteDroplet(id)
	if err != nil {
		return err
	}

	pred := func() bool {
		d, _, err := clst.client.GetDroplet(id)
		return err != nil || d == nil
	}
	return util.WaitFor(pred, 500*time.Millisecond, 1*time.Minute)
}

// SetACLs is not supported in DigitalOcean.
func (clst Cluster) SetACLs(acls []acl.ACL) error {
	log.Debug("DigitalOcean does not support ACLs")
	return nil
}

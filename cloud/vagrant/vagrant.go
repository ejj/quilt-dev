package vagrant

import (
	"errors"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/quilt/quilt/cloud/acl"
	"github.com/quilt/quilt/cloud/cfg"
	"github.com/quilt/quilt/cloud/machine"
	"github.com/quilt/quilt/counter"
	"github.com/satori/go.uuid"
)

// The Provider object represents a connection to Vagrant.
type Provider struct {
	namespace string
}

var c = counter.New("Vagrant")

// New creates a new vagrant provider.
func New(namespace string) (*Provider, error) {
	prvdr := Provider{namespace}
	err := addBox(box, "virtualbox")
	return &prvdr, err
}

// Boot creates instances in the `prvdr` configured according to the `bootSet`.
func (prvdr Provider) Boot(bootSet []machine.Machine) error {
	for _, m := range bootSet {
		if m.Preemptible {
			return errors.New(
				"vagrant does not support preemptible instances")
		}
	}

	// If any of the boot.Machine() calls fail, errChan will contain exactly one
	// error for this function to return.
	errChan := make(chan error, 1)

	var wg sync.WaitGroup
	for _, m := range bootSet {
		wg.Add(1)
		go func(m machine.Machine) {
			defer wg.Done()
			if err := bootMachine(m); err != nil {
				select {
				case errChan <- err:
				default:
				}
			}
		}(m)
	}
	wg.Wait()

	var err error
	select {
	case err = <-errChan:
	default:
	}

	return err
}

func bootMachine(m machine.Machine) error {
	id := uuid.NewV4().String()

	m.CfgOpts.MinionOpts.InboundPubIntf = inboundPublicInterface
	err := initMachine(cfg.Ubuntu(m.CfgOpts), m.Size, id)
	if err == nil {
		err = up(id)
	}

	if err != nil {
		destroy(id)
	}

	return err
}

// List queries `prvdr` for the list of booted machines.
func (prvdr Provider) List() ([]machine.Machine, error) {
	machines := []machine.Machine{}
	instanceIDs, err := list()

	if err != nil {
		return machines, err
	} else if len(instanceIDs) == 0 {
		return machines, nil
	}

	for _, instanceID := range instanceIDs {
		ip, err := publicIP(instanceID)
		if err != nil {
			log.WithError(err).Infof(
				"Failed to retrieve IP address for %s.",
				instanceID)
		}
		instance := machine.Machine{
			ID:        instanceID,
			PublicIP:  ip,
			PrivateIP: ip,
			Size:      size(instanceID),
		}
		machines = append(machines, instance)
	}
	return machines, nil
}

// Stop shuts down `machines` in `prvdr.
func (prvdr Provider) Stop(machines []machine.Machine) error {
	if machines == nil {
		return nil
	}
	for _, m := range machines {
		err := destroy(m.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetACLs is a noop for vagrant.
func (prvdr Provider) SetACLs(acls []acl.ACL) error {
	return nil
}

// UpdateFloatingIPs is not supported.
func (prvdr *Provider) UpdateFloatingIPs([]machine.Machine) error {
	return errors.New("vagrant provider does not support floating IPs")
}

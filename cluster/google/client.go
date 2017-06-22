package google

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	compute "google.golang.org/api/compute/v1"

	"github.com/quilt/quilt/util"
)

//go:generate mockery -inpkg -testonly -name=client
type client interface {
	GetInstance(zone, id string) (*compute.Instance, error)
	ListInstances(zone string, opts apiOptions) (*compute.InstanceList,
		error)
	InsertInstance(zone string, instance *compute.Instance) (
		*compute.Operation, error)
	DeleteInstance(zone, operation string) (*compute.Operation, error)
	AddAccessConfig(zone, instance, networkInterface string,
		accessConfig *compute.AccessConfig) (*compute.Operation, error)
	DeleteAccessConfig(zone, instance, accessConfig,
		networkInterface string) (*compute.Operation, error)
	GetZoneOperation(zone, operation string) (*compute.Operation, error)
	GetGlobalOperation(operation string) (*compute.Operation, error)
	ListFirewalls() (*compute.FirewallList, error)
	InsertFirewall(firewall *compute.Firewall) (
		*compute.Operation, error)
	PatchFirewall(name string, firewall *compute.Firewall) (
		*compute.Operation, error)
	DeleteFirewall(firewall string) (*compute.Operation, error)
	ListNetworks() (*compute.NetworkList, error)
	InsertNetwork(network *compute.Network) (
		*compute.Operation, error)
}

type clientImpl struct {
	gce    *compute.Service
	projID string
}

func newClient() (*clientImpl, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".gce", "quilt.json")
	configStr, err := util.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	service, err := newComputeService(configStr)
	if err != nil {
		return nil, err
	}

	projID, err := getProjectID(configStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get project ID: %s", err)
	}

	return &clientImpl{gce: service, projID: projID}, nil
}

const projectIDKey = "project_id"

func getProjectID(configStr string) (string, error) {
	configFields := map[string]string{}
	if err := json.Unmarshal([]byte(configStr), &configFields); err != nil {
		return "", err
	}

	projID, ok := configFields[projectIDKey]
	if !ok {
		return "", fmt.Errorf("missing field: %s", projectIDKey)
	}

	return projID, nil
}

/**
 * Service: Instances
 */

func (ci *clientImpl) GetInstance(zone, id string) (*compute.Instance, error) {
	return ci.gce.Instances.Get(ci.projID, zone, id).Do()
}

type apiOptions struct {
	filter string
}

func (ci *clientImpl) ListInstances(zone string, opts apiOptions) (
	*compute.InstanceList, error) {
	call := ci.gce.Instances.List(ci.projID, zone)
	if opts.filter != "" {
		call = call.Filter(opts.filter)
	}

	return call.Do()
}

func (ci *clientImpl) InsertInstance(zone string, instance *compute.Instance) (
	*compute.Operation, error) {
	return ci.gce.Instances.Insert(ci.projID, zone, instance).Do()
}

func (ci *clientImpl) DeleteInstance(zone, instance string) (*compute.Operation,
	error) {
	return ci.gce.Instances.Delete(ci.projID, zone, instance).Do()
}

func (ci *clientImpl) AddAccessConfig(zone, instance, networkInterface string,
	accessConfig *compute.AccessConfig) (*compute.Operation, error) {
	return ci.gce.Instances.AddAccessConfig(ci.projID, zone, instance,
		networkInterface, accessConfig).Do()
}

func (ci *clientImpl) DeleteAccessConfig(zone, instance, accessConfig,
	networkInterface string) (*compute.Operation, error) {
	return ci.gce.Instances.DeleteAccessConfig(ci.projID, zone, instance,
		accessConfig, networkInterface).Do()
}

/**
 * Service: ZoneOperations
 */

func (ci *clientImpl) GetZoneOperation(zone, operation string) (
	*compute.Operation, error) {
	return ci.gce.ZoneOperations.Get(ci.projID, zone, operation).Do()
}

/**
 * Service: GlobalOperations
 */

func (ci *clientImpl) GetGlobalOperation(operation string) (*compute.Operation,
	error) {
	return ci.gce.GlobalOperations.Get(ci.projID, operation).Do()
}

/**
 * Service: Firewall
 */

func (ci *clientImpl) ListFirewalls() (*compute.FirewallList, error) {
	return ci.gce.Firewalls.List(ci.projID).Do()
}

func (ci *clientImpl) InsertFirewall(firewall *compute.Firewall) (
	*compute.Operation, error) {
	return ci.gce.Firewalls.Insert(ci.projID, firewall).Do()
}

func (ci *clientImpl) PatchFirewall(name string, firewall *compute.Firewall) (
	*compute.Operation, error) {
	return ci.gce.Firewalls.Patch(ci.projID, name, firewall).Do()
}

func (ci *clientImpl) DeleteFirewall(firewall string) (
	*compute.Operation, error) {
	return ci.gce.Firewalls.Delete(ci.projID, firewall).Do()
}

/**
 * Service: Networks
 */

func (ci *clientImpl) ListNetworks() (*compute.NetworkList, error) {
	return ci.gce.Networks.List(ci.projID).Do()
}

func (ci *clientImpl) InsertNetwork(network *compute.Network) (
	*compute.Operation, error) {
	return ci.gce.Networks.Insert(ci.projID, network).Do()
}

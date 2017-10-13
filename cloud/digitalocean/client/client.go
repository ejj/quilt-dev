//go:generate mockery -name=Client

package client

import (
	"context"
	"net/http"

	"github.com/digitalocean/godo"
	"github.com/kelda/kelda/counter"
)

// A Client for DigitalOcean's API. Used for unit testing.
type Client interface {
	CreateDroplet(*godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error)
	DeleteDroplet(int) (*godo.Response, error)
	GetDroplet(int) (*godo.Droplet, *godo.Response, error)
	ListDroplets(*godo.ListOptions) ([]godo.Droplet, *godo.Response, error)

	ListFloatingIPs(*godo.ListOptions) ([]godo.FloatingIP, *godo.Response, error)
	AssignFloatingIP(string, int) (*godo.Action, *godo.Response, error)
	UnassignFloatingIP(string) (*godo.Action, *godo.Response, error)
}

type client struct {
	droplets          godo.DropletsService
	floatingIPs       godo.FloatingIPsService
	floatingIPActions godo.FloatingIPActionsService
}

var c = counter.New("Digital Ocean")

func (client client) CreateDroplet(req *godo.DropletCreateRequest) (*godo.Droplet,
	*godo.Response, error) {
	c.Inc("Create Droplet")
	return client.droplets.Create(context.Background(), req)
}

func (client client) DeleteDroplet(id int) (*godo.Response, error) {
	c.Inc("Delete Droplet")
	return client.droplets.Delete(context.Background(), id)
}

func (client client) GetDroplet(id int) (*godo.Droplet, *godo.Response, error) {
	c.Inc("Get Droplet")
	return client.droplets.Get(context.Background(), id)
}

func (client client) ListDroplets(opt *godo.ListOptions) ([]godo.Droplet,
	*godo.Response, error) {
	c.Inc("List Droplets")
	return client.droplets.List(context.Background(), opt)
}

func (client client) ListFloatingIPs(opt *godo.ListOptions) ([]godo.FloatingIP,
	*godo.Response, error) {
	c.Inc("List Floating IPs")
	return client.floatingIPs.List(context.Background(), opt)
}

func (client client) AssignFloatingIP(ip string, id int) (*godo.Action,
	*godo.Response, error) {
	c.Inc("Assign Floating IP")
	return client.floatingIPActions.Assign(context.Background(), ip, id)
}

func (client client) UnassignFloatingIP(ip string) (*godo.Action, *godo.Response,
	error) {
	c.Inc("Remove Floating IP")
	return client.floatingIPActions.Unassign(context.Background(), ip)
}

// New creates a new DigitalOcean client.
func New(oauthClient *http.Client) Client {
	api := godo.NewClient(oauthClient)
	return client{
		droplets:          api.Droplets,
		floatingIPs:       api.FloatingIPs,
		floatingIPActions: api.FloatingIPActions,
	}
}

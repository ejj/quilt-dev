//go:generate mockery -name=I -outpkg=nlmock -output=./nlmock

package nl

import (
	"net"

	"github.com/kelda/kelda/counter"
	"github.com/vishvananda/netlink"
)

// I implements a mock interface netlink.
type I interface {
	AddVeth(name, peer string, mtu int) error
	LinkSetUp(link Link) error
	LinkDel(link Link) error
	LinkByName(name string) (Link, error)
	LinkByIndex(index int) (Link, error)
	AddrAdd(link Link, ip net.IPNet) error
	RouteList(family int) ([]Route, error)
}

// N holds a global instance of I.
var N I = n{}

type n struct{}

// Link wraps netlink.Link.
type Link netlink.Link

// Route wraps netlink.Route.
type Route netlink.Route

var c = counter.New("Netlink")

func (n n) AddVeth(name, peer string, mtu int) error {
	c.Inc("Add Veth")
	return netlink.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name, MTU: mtu},
		PeerName:  peer})
}

func (n n) LinkSetUp(link Link) error {
	c.Inc("Link Up")
	return netlink.LinkSetUp(link)
}

func (n n) LinkDel(link Link) error {
	c.Inc("Link Del")
	return netlink.LinkDel(link)
}

func (n n) LinkByName(name string) (Link, error) {
	c.Inc("Get Link")
	return netlink.LinkByName(name)
}

func (n n) LinkByIndex(index int) (Link, error) {
	c.Inc("Get Link")
	return netlink.LinkByIndex(index)
}

func (n n) AddrAdd(link Link, ip net.IPNet) error {
	c.Inc("Add Address")
	return netlink.AddrAdd(link, &netlink.Addr{IPNet: &ip})
}

func (n n) RouteList(family int) ([]Route, error) {
	c.Inc("List Routes")
	res, err := netlink.RouteList(nil, family)

	var routes []Route
	for _, r := range res {
		routes = append(routes, Route(r))
	}
	return routes, err
}

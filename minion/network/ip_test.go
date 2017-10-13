package network

import (
	"fmt"
	"net"
	"sort"
	"testing"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/minion/ipdef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeSubnetBlacklistError(t *testing.T) {
	t.Parallel()

	conn := db.New()
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMinion()
		m.Role = db.Worker
		m.HostSubnets = []string{
			"foo",
		}
		view.Commit(m)

		_, err := makeSubnetBlacklist(view)
		assert.EqualError(t, err, "parse subnet foo: invalid CIDR address: foo")
		return nil
	})
}

func TestMakeSubnetBlacklist(t *testing.T) {
	t.Parallel()

	conn := db.New()
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMinion()
		m.Role = db.Worker
		m.HostSubnets = []string{
			"10.0.1.0/24",
			"10.0.2.0/24",
			"172.0.0.0/8",
		}
		view.Commit(m)

		m = view.InsertMinion()
		m.Role = db.Worker
		m.HostSubnets = []string{
			"10.0.1.0/24",
			"10.0.3.0/24",
		}
		view.Commit(m)

		blacklist, err := makeSubnetBlacklist(view)
		assert.NoError(t, err)

		// Convert the blacklist back into strings for comparison.
		var blacklistStr []string
		for _, subnet := range blacklist {
			blacklistStr = append(blacklistStr, subnet.String())
		}
		sort.Strings(blacklistStr)
		assert.Equal(t, []string{"10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"},
			blacklistStr)
		return nil
	})
}

func TestMakeIPContext(t *testing.T) {
	t.Parallel()

	subnetBlacklist := []net.IPNet{
		{
			IP:   net.IPv4(10, 0, 1, 0),
			Mask: net.CIDRMask(24, 32),
		},
		{
			IP:   net.IPv4(10, 0, 2, 0),
			Mask: net.CIDRMask(24, 32),
		},
	}

	conn := db.New()
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		// A container with an IP address.
		dbc := view.InsertContainer()
		dbc.IP = "10.0.0.2"
		dbc.BlueprintID = "1"
		view.Commit(dbc)

		// A container without an IP address.
		dbc = view.InsertContainer()
		dbc.BlueprintID = "2"
		view.Commit(dbc)

		// A container with an IP in a blacklisted subnet.
		dbc = view.InsertContainer()
		dbc.IP = "10.0.2.1"
		dbc.BlueprintID = "3"
		view.Commit(dbc)

		// A load balancer with an IP address.
		loadBalancer := view.InsertLoadBalancer()
		loadBalancer.Name = "yellow"
		loadBalancer.IP = "10.0.0.3"
		view.Commit(loadBalancer)

		// A load balancer without an IP address.
		loadBalancer = view.InsertLoadBalancer()
		loadBalancer.Name = "blue"
		view.Commit(loadBalancer)

		// A load balancer with an IP in a blacklisted subnet.
		loadBalancer = view.InsertLoadBalancer()
		loadBalancer.Name = "green"
		loadBalancer.IP = "10.0.1.1"
		view.Commit(loadBalancer)

		return nil
	})

	var ctx ipContext
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		ctx = makeIPContext(view, subnetBlacklist)
		return nil
	})

	assert.Equal(t, map[string]struct{}{
		"10.0.0.0": {},
		"10.0.0.1": {},
		"10.0.0.2": {},
		"10.0.0.3": {},
	}, ctx.reserved)

	assert.Len(t, ctx.unassignedContainers, 2)
	assert.Contains(t, ctx.unassignedContainers,
		db.Container{ID: 2, BlueprintID: "2"})
	assert.Contains(t, ctx.unassignedContainers,
		db.Container{ID: 3, BlueprintID: "3"})

	assert.Len(t, ctx.unassignedLoadBalancers, 2)
	assert.Contains(t, ctx.unassignedLoadBalancers,
		db.LoadBalancer{ID: 5, Name: "blue"})
	assert.Contains(t, ctx.unassignedLoadBalancers,
		db.LoadBalancer{ID: 6, Name: "green"})
}

func TestAllocateContainerIPs(t *testing.T) {
	t.Parallel()
	conn := db.New()

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		dbc := view.InsertContainer()
		dbc.IP = "10.0.0.2"
		dbc.BlueprintID = "1"
		view.Commit(dbc)

		dbc = view.InsertContainer()
		dbc.BlueprintID = "2"
		view.Commit(dbc)

		ctx := ipContext{
			reserved:             map[string]struct{}{},
			unassignedContainers: []db.Container{dbc},
		}
		allocateContainerIPs(view, ctx)
		return nil
	})

	dbcs := conn.SelectFromContainer(nil)
	assert.Len(t, dbcs, 2)

	sort.Sort(db.ContainerSlice(dbcs))

	dbc := dbcs[0]
	dbc.ID = 0
	assert.Equal(t, db.Container{IP: "10.0.0.2", BlueprintID: "1"}, dbc)

	dbc = dbcs[1]
	assert.Equal(t, "2", dbc.BlueprintID)
	assert.True(t, ipdef.QuiltSubnet.Contains(net.ParseIP(dbc.IP)))
}

func TestAllocateLoadBalancerIPs(t *testing.T) {
	t.Parallel()
	conn := db.New()

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		lb := view.InsertLoadBalancer()
		lb.Name = "yellow"
		view.Commit(lb)

		ctx := ipContext{
			reserved:                map[string]struct{}{},
			unassignedLoadBalancers: []db.LoadBalancer{lb},
		}
		assert.NoError(t, allocateLoadBalancerIPs(view, ctx))
		return nil
	})

	loadBalancers := conn.SelectFromLoadBalancer(nil)
	assert.Len(t, loadBalancers, 1)
	lbIP := net.ParseIP(loadBalancers[0].IP)
	assert.True(t, ipdef.QuiltSubnet.Contains(lbIP))
}

func TestAllocate(t *testing.T) {
	t.Parallel()

	subnet := net.IPNet{
		IP:   net.IPv4(0xab, 0xcd, 0xe0, 0x00),
		Mask: net.CIDRMask(20, 32),
	}
	conflicts := map[string]struct{}{}
	ipSet := map[string]struct{}{}

	// Only 4k IPs, in 0xfffff000. Guaranteed a collision
	for i := 0; i < 5000; i++ {
		ip, err := allocateIP(ipSet, subnet)
		if err != nil {
			continue
		}

		if _, ok := conflicts[ip]; ok {
			t.Fatalf("IP Double allocation: 0x%x", ip)
		}

		require.True(t, subnet.Contains(net.ParseIP(ip)),
			fmt.Sprintf("\"%s\" is not in %s", ip, subnet))
		conflicts[ip] = struct{}{}
	}

	assert.Equal(t, len(conflicts), len(ipSet))
	if len(conflicts) < 2500 || len(conflicts) > 4096 {
		// If the code's working, this is possible but *extremely* unlikely.
		// Probably a bug.
		t.Errorf("Too few conflicts: %d", len(conflicts))
	}
}

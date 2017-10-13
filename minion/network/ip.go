package network

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/minion/ipdef"

	log "github.com/sirupsen/logrus"
)

/* runUpdateIPs allocates IPs to containers and load balancers.

XXX: It takes into account the subnets governed by routes on the host network
stack of worker machines. If any worker has a route that intersects with the
Quilt container subnet, we refuse to allocate container IPs within that subnet.
If a container had an IP within a non-Quilt route, traffic destined to it from
the host might get routed to the wrong interface.

Note that the proper fix to this problem is to separate the Quilt networking
stack from the host network. */
func runUpdateIPs(conn db.Conn) {
	for range conn.Trigger(db.ContainerTable, db.LoadBalancerTable, db.EtcdTable,
		db.MinionTable).C {
		if !conn.EtcdLeader() {
			continue
		}

		err := conn.Txn(db.ContainerTable, db.LoadBalancerTable,
			db.MinionTable).Run(updateIPsOnce)
		if err != nil {
			log.WithError(err).Warn("Failed to allocate IP addresses")
		}
	}
}

// ipContext describes what addresses have been allocated, and what entities
// require new IP addresses.
type ipContext struct {
	reserved map[string]struct{}

	unassignedContainers    []db.Container
	unassignedLoadBalancers []db.LoadBalancer
}

func makeIPContext(view db.Database, subnetBlacklist []net.IPNet) ipContext {
	ctx := ipContext{
		reserved: map[string]struct{}{
			ipdef.GatewayIP.String():      {},
			ipdef.LoadBalancerIP.String(): {},

			// While not strictly required, it would be odd to allocate
			// 10.0.0.0.
			ipdef.QuiltSubnet.IP.String(): {},
		},
	}

	for _, dbc := range view.SelectFromContainer(nil) {
		if dbc.IP != "" && ipBlacklisted(dbc.IP, subnetBlacklist) {
			dbc.IP = ""
		}

		if dbc.IP != "" {
			ctx.reserved[dbc.IP] = struct{}{}
		} else {
			ctx.unassignedContainers = append(ctx.unassignedContainers, dbc)
		}
	}

	for _, dbl := range view.SelectFromLoadBalancer(nil) {
		if dbl.IP != "" && ipBlacklisted(dbl.IP, subnetBlacklist) {
			dbl.IP = ""
		}

		if dbl.IP != "" {
			ctx.reserved[dbl.IP] = struct{}{}
		} else {
			ctx.unassignedLoadBalancers = append(ctx.unassignedLoadBalancers,
				dbl)
		}
	}

	return ctx
}

func updateIPsOnce(view db.Database) error {
	subnetBlacklist, err := makeSubnetBlacklist(view)
	if err != nil {
		return fmt.Errorf("make subnet blacklist: %s", err)
	}

	// Attempt to allocate IPs up to three times in case we happen to choose
	// an IP that falls within a blacklisted subnet.
	for i := 0; i < 3; i++ {
		ctx := makeIPContext(view, subnetBlacklist)
		if len(ctx.unassignedContainers) == 0 &&
			len(ctx.unassignedLoadBalancers) == 0 {
			return nil
		}

		err = allocateContainerIPs(view, ctx)
		if err == nil {
			err = allocateLoadBalancerIPs(view, ctx)
		}
	}
	return err
}

// makeSubnetBlacklist returns all subnets that are governed by routes in a
// worker machine's network stack, and intersect with the Quilt container
// subnet.
func makeSubnetBlacklist(view db.Database) ([]net.IPNet, error) {
	isWorker := func(m db.Minion) bool {
		return m.Role == db.Worker
	}

	subnets := map[string]struct{}{}
	for _, m := range view.SelectFromMinion(isWorker) {
		for _, subnet := range m.HostSubnets {
			subnets[subnet] = struct{}{}
		}
	}

	var subnetBlacklist []net.IPNet
	for subnetStr := range subnets {
		_, subnet, err := net.ParseCIDR(subnetStr)
		if err != nil {
			return nil, fmt.Errorf("parse subnet %s: %s", subnetStr, err)
		}

		if subnetIntersects(ipdef.QuiltSubnet, *subnet) {
			subnetBlacklist = append(subnetBlacklist, *subnet)
		}
	}
	return subnetBlacklist, nil
}

func subnetIntersects(a net.IPNet, b net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

func ipBlacklisted(ip string, subnetBlacklist []net.IPNet) bool {
	for _, subnet := range subnetBlacklist {
		if subnet.Contains(net.ParseIP(ip)) {
			return true
		}
	}
	return false
}

func allocateContainerIPs(view db.Database, ctx ipContext) error {
	for _, dbc := range ctx.unassignedContainers {
		c.Inc("Allocate Container IP")
		ip, err := allocateIP(ctx.reserved, ipdef.QuiltSubnet)
		if err != nil {
			return err
		}

		dbc.IP = ip
		view.Commit(dbc)
	}

	return nil
}

func allocateLoadBalancerIPs(view db.Database, ctx ipContext) error {
	for _, lb := range ctx.unassignedLoadBalancers {
		c.Inc("Allocate LoadBalancer IP")
		ip, err := allocateIP(ctx.reserved, ipdef.QuiltSubnet)
		if err != nil {
			return err
		}

		lb.IP = ip
		view.Commit(lb)
	}

	return nil
}

func allocateIP(ipSet map[string]struct{}, subnet net.IPNet) (string, error) {
	prefix := binary.BigEndian.Uint32(subnet.IP.To4())
	mask := binary.BigEndian.Uint32(subnet.Mask)

	randStart := rand32() & ^mask
	for offset := uint32(0); offset <= ^mask; offset++ {

		randIP32 := ((randStart + offset) & ^mask) | (prefix & mask)

		randIP := net.IP(make([]byte, 4))
		binary.BigEndian.PutUint32(randIP, randIP32)
		randIPStr := randIP.String()

		if _, ok := ipSet[randIPStr]; !ok {
			ipSet[randIPStr] = struct{}{}
			return randIPStr, nil
		}
	}
	return "", errors.New("IP pool exhausted")
}

var rand32 = rand.Uint32

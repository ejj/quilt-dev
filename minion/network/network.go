// Package network manages the network services of the application dataplane.  This
// means ensuring that containers can find and communicate with each other in accordance
// with the policy specification.  It achieves this by manipulating IP addresses and
// hostnames within the containers, Open vSwitch on each running worker, and the OVN
// controller.
package network

import (
	"github.com/quilt/quilt/counter"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/join"
	"github.com/quilt/quilt/minion/ipdef"
	"github.com/quilt/quilt/minion/ovsdb"

	log "github.com/Sirupsen/logrus"
)

const (
	lSwitch                = "quilt"
	loadBalancerRouter     = "loadBalancerRouter"
	loadBalancerSwitchPort = "loadBalancerSwitchPort"
	loadBalancerRouterPort = "loadBalancerRouterPort"
)

var c = counter.New("Network")

// Run blocks implementing the network services.
func Run(conn db.Conn, inboundPubIntf, outboundPubIntf string) {
	go runNat(conn, inboundPubIntf, outboundPubIntf)
	go runDNS(conn)
	go runUpdateIPs(conn)

	for range conn.TriggerTick(30, db.MinionTable, db.ContainerTable,
		db.ConnectionTable, db.LabelTable, db.EtcdTable).C {
		if conn.EtcdLeader() {
			runMaster(conn)
		}
	}
}

// The leader of the cluster is responsible for properly configuring OVN northd
// for container networking and load balancing.  This means creating a logical
// port for each container, creating ACLs, creating the load balancer router,
// and creating load balancers.  The specialized OpenFlow rules Quilt requires
// are managed by the workers individuallly.
func runMaster(conn db.Conn) {
	c.Inc("Run Master")

	var labels []db.Label
	var containers []db.Container
	var connections []db.Connection
	conn.Txn(db.ConnectionTable, db.ContainerTable, db.EtcdTable,
		db.LabelTable, db.MinionTable).Run(func(view db.Database) error {

		labels = view.SelectFromLabel(func(label db.Label) bool {
			return label.IP != ""
		})

		containers = view.SelectFromContainer(func(dbc db.Container) bool {
			return dbc.IP != ""
		})

		connections = view.SelectFromConnection(nil)
		return nil
	})

	ovsdbClient, err := ovsdb.Open()
	if err != nil {
		log.WithError(err).Error("Failed to connect to OVSDB.")
		return
	}
	defer ovsdbClient.Disconnect()

	updateLogicalSwitch(ovsdbClient, containers)
	updateLoadBalancerRouter(ovsdbClient)
	updateLoadBalancers(ovsdbClient, labels)
	updateACLs(ovsdbClient, connections, labels)
}

func updateLogicalSwitch(ovsdbClient ovsdb.Client, containers []db.Container) {
	switchExists, err := ovsdbClient.LogicalSwitchExists(lSwitch)
	if err != nil {
		log.WithError(err).Error("Failed to check existence of logical switch")
		return
	}

	if !switchExists {
		if err := ovsdbClient.CreateLogicalSwitch(lSwitch); err != nil {
			log.WithError(err).Error("Failed to create logical switch")
			return
		}
	}

	lports, err := ovsdbClient.ListSwitchPorts()
	if err != nil {
		log.WithError(err).Error("Failed to list OVN switch ports.")
		return
	}

	expPorts := []ovsdb.SwitchPort{
		{
			Name: loadBalancerSwitchPort,
			Type: "router",
			Options: map[string]string{
				"router-port": loadBalancerRouterPort,
			},
			// The addresses field is handled by `updateLoadBalancerARP`.
		},
	}
	for _, dbc := range containers {
		expPorts = append(expPorts, ovsdb.SwitchPort{
			Name: dbc.IP,
			// OVN represents network interfaces with the empty string.
			Type:      "",
			Addresses: []string{ipdef.IPStrToMac(dbc.IP) + " " + dbc.IP},
		})
	}

	key := func(intf interface{}) interface{} {
		return intf.(ovsdb.SwitchPort).Name
	}
	_, toAdd, toDel := join.HashJoin(ovsdb.SwitchPortSlice(expPorts),
		ovsdb.SwitchPortSlice(lports), key, key)

	for _, intf := range toAdd {
		lport := intf.(ovsdb.SwitchPort)
		err := ovsdbClient.CreateSwitchPort(lSwitch, lport)
		if err != nil {
			log.WithError(err).Warnf(
				"Failed to create logical switch port: %s", lport.Name)
		} else {
			log.Infof("New logical switch port: %s", lport.Name)
		}
	}

	for _, intf := range toDel {
		lport := intf.(ovsdb.SwitchPort)
		if err := ovsdbClient.DeleteSwitchPort(lSwitch, lport); err != nil {
			log.WithError(err).Warnf(
				"Failed to delete logical switch port: %s", lport.Name)
		} else {
			log.Infof("Delete logical switch port: %s", lport.Name)
		}
	}
}

func updateLoadBalancerRouter(ovsdbClient ovsdb.Client) {
	routerExists, err := ovsdbClient.LogicalRouterExists(loadBalancerRouter)
	if err != nil {
		log.WithError(err).Error(
			"Failed to check existence of load balancer router")
		return
	}

	if !routerExists {
		err := ovsdbClient.CreateLogicalRouter(loadBalancerRouter)
		if err != nil {
			log.WithError(err).Error("Failed to create load balancer router")
			return
		}
	}

	lports, err := ovsdbClient.ListRouterPorts()
	if err != nil {
		log.WithError(err).Error("Failed to list OVN router ports")
		return
	}

	expPorts := []ovsdb.RouterPort{
		{
			Name:     loadBalancerRouterPort,
			MAC:      ipdef.LoadBalancerMac,
			Networks: []string{ipdef.QuiltSubnet.String()},
		},
	}

	key := func(intf interface{}) interface{} {
		return intf.(ovsdb.RouterPort).Name
	}
	_, toAdd, toDel := join.HashJoin(ovsdb.RouterPortSlice(expPorts),
		ovsdb.RouterPortSlice(lports), key, key)

	for _, intf := range toAdd {
		lport := intf.(ovsdb.RouterPort)
		err := ovsdbClient.CreateRouterPort(loadBalancerRouter, lport)
		if err != nil {
			log.WithError(err).WithField("lport", lport).
				Warnf("Failed to create logical router port.")
		} else {
			log.Infof("New logical router port: %s", lport.Name)
		}
	}

	for _, intf := range toDel {
		lport := intf.(ovsdb.RouterPort)
		err := ovsdbClient.DeleteRouterPort(loadBalancerRouter, lport)
		if err != nil {
			log.WithError(err).Warnf(
				"Failed to delete logical router port: %s", lport.Name)
		} else {
			log.Infof("Delete logical router port: %s", lport.Name)
		}
	}
}

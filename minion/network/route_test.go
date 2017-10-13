package network

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/minion/ipdef"
	"github.com/kelda/kelda/minion/nl"
	"github.com/kelda/kelda/minion/nl/nlmock"
)

func TestWriteSubnets(t *testing.T) {
	mockNetlink := new(nlmock.I)
	nl.N = mockNetlink

	conn := db.New()
	conn.Txn(db.MinionTable).Run(func(view db.Database) error {
		m := view.InsertMinion()
		m.Self = true
		view.Commit(m)
		return nil
	})

	mockNetlink.On("RouteList", mock.Anything).Once().Return(nil, errors.New("err"))
	assert.EqualError(t, writeSubnetsOnce(conn), "list routes: err")

	// Test an error getting a link.
	mockNetlink.On("RouteList", mock.Anything).Once().Return(
		[]nl.Route{{LinkIndex: 3}}, nil)
	mockNetlink.On("LinkByIndex", 1).Return(&netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: "quilt-int",
		},
	}, nil)
	mockNetlink.On("LinkByIndex", 2).Return(&netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: "2",
		},
	}, nil)
	mockNetlink.On("LinkByIndex", 3).Return(nil, errors.New("unknown"))
	assert.EqualError(t, writeSubnetsOnce(conn), "get link: unknown")

	mockNetlink.On("RouteList", mock.Anything).Once().Return([]nl.Route{
		{
			LinkIndex: 1,
			Dst:       &ipdef.QuiltSubnet,
		},
		{
			LinkIndex: 2,
			Dst: &net.IPNet{
				IP:   net.IPv4(10, 0, 1, 0),
				Mask: net.CIDRMask(24, 32),
			},
		},
	}, nil)
	assert.NoError(t, writeSubnetsOnce(conn))
	assert.Equal(t, []string{"10.0.1.0/24"}, conn.MinionSelf().HostSubnets)
}

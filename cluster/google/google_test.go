package google

import (
	"errors"
	"testing"

	"github.com/quilt/quilt/cluster/acl"
	"github.com/quilt/quilt/cluster/google/client/mocks"
	"github.com/quilt/quilt/cluster/machine"
	"github.com/stretchr/testify/suite"

	compute "google.golang.org/api/compute/v1"
)

type GoogleTestSuite struct {
	suite.Suite

	gce  *mocks.Client
	clst *Cluster
}

func (s *GoogleTestSuite) SetupTest() {
	s.gce = new(mocks.Client)
	s.clst = &Cluster{
		gce:  s.gce,
		ns:   "namespace",
		zone: "zone-1",
	}
}

func (s *GoogleTestSuite) TestList() {
	s.gce.On("ListInstances", "zone-1",
		"description eq namespace").Return(&compute.InstanceList{
		Items: []*compute.Instance{
			{
				MachineType: "machine/split/type-1",
				Name:        "name-1",
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						AccessConfigs: []*compute.AccessConfig{
							{
								NatIP: "x.x.x.x",
							},
						},
						NetworkIP: "y.y.y.y",
					},
				},
			},
		},
	}, nil)

	machines, err := s.clst.List()
	s.NoError(err)
	s.Len(machines, 1)
	s.Equal(machines[0], machine.Machine{
		ID:        "name-1",
		PublicIP:  "x.x.x.x",
		PrivateIP: "y.y.y.y",
		Size:      "type-1",
	})
}

func (s *GoogleTestSuite) TestListFirewalls() {
	s.clst.networkName = "network"
	s.clst.intFW = "intFW"

	s.gce.On("ListFirewalls").Return(&compute.FirewallList{
		Items: []*compute.Firewall{
			{
				Network:    networkURL(s.clst.networkName),
				Name:       "badZone",
				TargetTags: []string{"zone-2"},
			},
			{
				Network:    networkURL(s.clst.networkName),
				Name:       "intFW",
				TargetTags: []string{"zone-1"},
			},
			{
				Network:    networkURL("ignoreMe"),
				Name:       "badNetwork",
				TargetTags: []string{"zone-1"},
			},
			{
				Network:    networkURL(s.clst.networkName),
				Name:       "shouldReturn",
				TargetTags: []string{"zone-1"},
			},
		},
	}, nil).Once()

	fws, err := s.clst.listFirewalls()
	s.NoError(err)
	s.Len(fws, 1)
	s.Equal(fws[0].Name, "shouldReturn")

	s.gce.On("ListFirewalls").Return(nil, errors.New("err")).Once()
	_, err = s.clst.listFirewalls()
	s.EqualError(err, "list firewalls: err")
}

func (s *GoogleTestSuite) TestParseACLs() {
	parsed, err := s.clst.parseACLs([]compute.Firewall{
		{
			Name: "firewall",
			Allowed: []*compute.FirewallAllowed{
				{Ports: []string{"80", "20-25"}},
			},
			SourceRanges: []string{"foo", "bar"},
		},
		{
			Name: "firewall2",
			Allowed: []*compute.FirewallAllowed{
				{Ports: []string{"1-65535"}},
			},
			SourceRanges: []string{"foo"},
		},
	})
	s.NoError(err)
	s.Equal([]acl.ACL{
		{MinPort: 80, MaxPort: 80, CidrIP: "foo"},
		{MinPort: 20, MaxPort: 25, CidrIP: "foo"},
		{MinPort: 80, MaxPort: 80, CidrIP: "bar"},
		{MinPort: 20, MaxPort: 25, CidrIP: "bar"},
		{MinPort: 1, MaxPort: 65535, CidrIP: "foo"},
	}, parsed)

	_, err = s.clst.parseACLs([]compute.Firewall{
		{
			Name: "firewall",
			Allowed: []*compute.FirewallAllowed{
				{Ports: []string{"NaN"}},
			},
			SourceRanges: []string{"foo"},
		},
	})
	s.EqualError(err, `parse ports of firewall: parse ints: strconv.Atoi: `+
		`parsing "NaN": invalid syntax`)

	_, err = s.clst.parseACLs([]compute.Firewall{
		{
			Name: "firewall",
			Allowed: []*compute.FirewallAllowed{
				{Ports: []string{"1-80-81"}},
			},
			SourceRanges: []string{"foo"},
		},
	})
	s.EqualError(err, "parse ports of firewall: unrecognized port format: 1-80-81")
}

func TestGoogleTestSuite(t *testing.T) {
	suite.Run(t, new(GoogleTestSuite))
}

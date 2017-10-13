package network

import (
	"net"
	"testing"

	"github.com/kelda/kelda/db"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestUpdateTable(t *testing.T) {
	t.Parallel()

	listenAndServe = func(table *dnsTable) error { return assert.AnError }
	assert.Nil(t, updateTable(nil, nil))

	listenAndServe = func(table *dnsTable) error {
		table.server.NotifyStartedFunc()
		return nil
	}

	table := updateTable(nil, []db.Hostname{{Hostname: "foo", IP: "1.2.3.4"}})
	assert.NotNil(t, table)
	assert.Equal(t, map[string]net.IP{"foo.q.": net.IPv4(1, 2, 3, 4)}, table.records)

	newTable := updateTable(table, []db.Hostname{{Hostname: "foo", IP: "5.6.7.8"}})
	assert.NotNil(t, newTable)
	assert.True(t, table == newTable) // Pointer Equality.
	assert.Equal(t, map[string]net.IP{"foo.q.": net.IPv4(5, 6, 7, 8)},
		newTable.records)
}

func TestGenResponse(t *testing.T) {
	t.Parallel()

	table := makeTable(map[string]net.IP{
		"a.q.": net.IPv4(1, 2, 3, 4),
	})

	req := &dns.Msg{}
	req.SetQuestion("foo.", dns.TypeAAAA)
	resp := table.genResponse(req)
	assert.Equal(t, req.Id, resp.Id)
	assert.Equal(t, resp.Rcode, dns.RcodeNotImplemented)

	req.Question = nil
	resp = table.genResponse(req)
	assert.Equal(t, req.Id, resp.Id)
	assert.Equal(t, dns.RcodeNotImplemented, resp.Rcode)

	req.SetQuestion("bad.q.", dns.TypeA)
	resp = table.genResponse(req)
	assert.Nil(t, resp)

	req.SetQuestion("a.q.", dns.TypeA)
	resp = table.genResponse(req)
	exp := *req
	exp.Response = true
	exp.Rcode = dns.RcodeSuccess
	exp.Answer = []dns.RR{&dns.A{
		Hdr: dns.RR_Header{
			Name:   "a.q.",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    dnsTTL,
		},
		A: net.IPv4(1, 2, 3, 4),
	}}
	assert.Equal(t, &exp, resp)

}

func TestLookupA(t *testing.T) {
	t.Parallel()

	table := makeTable(map[string]net.IP{
		"a.q.": net.IPv4(1, 2, 3, 4),
	})

	assert.Empty(t, table.lookupA("bad.q."))
	assert.Equal(t, []net.IP{net.IPv4(1, 2, 3, 4)}, table.lookupA("a.q."))

	lookupHost = func(string) ([]string, error) { return nil, assert.AnError }
	assert.Empty(t, table.lookupA("quilt.io."))

	lookupHost = func(string) ([]string, error) { return []string{"bad"}, nil }
	assert.Empty(t, table.lookupA("quilt.io."))

	lookupHost = func(string) ([]string, error) {
		return []string{"2601:644:380:cde:fc06:2533:adf9:2891"}, nil
	}
	assert.Empty(t, table.lookupA("quilt.io."))

	lookupHost = func(string) ([]string, error) {
		return []string{"1.2.3.4", "5.6.7.8"}, nil
	}
	assert.Equal(t, []net.IP{net.IPv4(1, 2, 3, 4), net.IPv4(5, 6, 7, 8)},
		table.lookupA("quilt.io."))
}

func TestMakeTable(t *testing.T) {
	t.Parallel()

	records := map[string]net.IP{"a": net.IPv4(1, 2, 3, 4)}
	tbl := makeTable(records)
	assert.Equal(t, tbl.records, records)
	assert.Equal(t, tbl.server.Addr, "10.0.0.1:53")
	assert.Equal(t, tbl.server.Net, "udp")
}

func TestHostnamesToDNS(t *testing.T) {
	t.Parallel()

	res := hostnamesToDNS([]db.Hostname{{
		Hostname: "h1",
	}, {
		Hostname: "h2",
		IP:       "badIP",
	}, {
		Hostname: "h3",
		IP:       "1.2.3.4",
	}, {
		Hostname: "h4",
		IP:       "5.6.7.8",
	}, {
		Hostname: "1.h4",
		IP:       "1.1.1.1",
	}, {
		Hostname: "2.h4",
		IP:       "2.2.2.2",
	}})
	exp := map[string]net.IP{
		"h3.q.":   net.IPv4(1, 2, 3, 4),
		"h4.q.":   net.IPv4(5, 6, 7, 8),
		"1.h4.q.": net.IPv4(1, 1, 1, 1),
		"2.h4.q.": net.IPv4(2, 2, 2, 2),
	}
	assert.Equal(t, exp, res)
}

func TestSyncHostnamesWorker(t *testing.T) {
	conn := db.New()
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		dbl := view.InsertLoadBalancer()
		dbl.Name = "lb"
		dbl.IP = "IP"
		view.Commit(dbl)
		return nil
	})
	syncHostnamesOnce(conn)
	assert.Empty(t, conn.SelectFromHostname(nil))

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		etcd := view.InsertEtcd()
		etcd.Leader = true
		view.Commit(etcd)
		return nil
	})
	syncHostnamesOnce(conn)
	assert.Equal(t, []db.Hostname{{ID: 3, Hostname: "lb", IP: "IP"}},
		conn.SelectFromHostname(nil))
}

type syncHostnameTest struct {
	loadBalancers              []db.LoadBalancer
	containers                 []db.Container
	oldHostnames, expHostnames []db.Hostname
}

func TestSyncHostnames(t *testing.T) {
	tests := []syncHostnameTest{
		{
			loadBalancers: []db.LoadBalancer{
				{
					Name: "foo",
					IP:   "fooIP",
				},
			},
			expHostnames: []db.Hostname{
				{Hostname: "foo", IP: "fooIP"},
			},
		},
		{
			loadBalancers: []db.LoadBalancer{
				{
					Name: "foo",
					IP:   "fooIP",
				},
				{
					Name: "bar",
					IP:   "barIP",
				},
			},
			oldHostnames: []db.Hostname{
				{Hostname: "foo", IP: "fooIP"},
			},
			expHostnames: []db.Hostname{
				{Hostname: "foo", IP: "fooIP"},
				{Hostname: "bar", IP: "barIP"},
			},
		},
		{
			loadBalancers: []db.LoadBalancer{
				{
					Name: "bar",
					IP:   "barIP",
				},
			},
			oldHostnames: []db.Hostname{
				{Hostname: "foo", IP: "fooIP"},
				{Hostname: "bar", IP: "barIP"},
			},
			expHostnames: []db.Hostname{
				{Hostname: "bar", IP: "barIP"},
			},
		},
		{
			loadBalancers: []db.LoadBalancer{
				{
					Name: "foo",
					IP:   "fooIP",
				},
			},
			containers: []db.Container{
				{
					Hostname: "container",
					IP:       "containerIP",
				},
			},
			oldHostnames: []db.Hostname{
				{Hostname: "foo", IP: "fooIP"},
			},
			expHostnames: []db.Hostname{
				{Hostname: "foo", IP: "fooIP"},
				{Hostname: "container", IP: "containerIP"},
			},
		},
	}
	for _, test := range tests {
		conn := db.New()
		conn.Txn(db.AllTables...).Run(func(view db.Database) error {
			etcd := view.InsertEtcd()
			etcd.Leader = true
			view.Commit(etcd)

			for _, l := range test.loadBalancers {
				dbl := view.InsertLoadBalancer()
				l.ID = dbl.ID
				view.Commit(l)
			}
			for _, h := range test.oldHostnames {
				dbh := view.InsertHostname()
				h.ID = dbh.ID
				view.Commit(h)
			}
			for _, c := range test.containers {
				dbc := view.InsertContainer()
				c.ID = dbc.ID
				view.Commit(c)
			}
			return nil
		})
		syncHostnamesOnce(conn)
		assertHostnamesEqual(t, test.expHostnames, conn.SelectFromHostname(nil))
	}
}

func assertHostnamesEqual(t *testing.T, exp, actual []db.Hostname) {
	assert.Len(t, actual, len(exp))
	assert.Equal(t, toHostnameMap(exp), toHostnameMap(actual))
}

func toHostnameMap(hostnames []db.Hostname) map[db.Hostname]struct{} {
	hostnameMap := map[db.Hostname]struct{}{}
	for _, h := range hostnames {
		h.ID = 0
		hostnameMap[h] = struct{}{}
	}
	return hostnameMap
}

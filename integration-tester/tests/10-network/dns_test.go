package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kelda/kelda/api/client"
	"github.com/kelda/kelda/db"
)

// anyIPAllowed is used to indicate that any non-error response is okay for an external
// DNS query.
const anyIPAllowed = "0.0.0.0"

var externalHostnames = []string{"google.com", "facebook.com", "en.wikipedia.org"}

type dnsTester struct {
	hostnameIPMap map[string]string
}

func newDNSTester(clnt client.Client) (dnsTester, error) {
	loadBalancers, err := clnt.QueryLoadBalancers()
	if err != nil {
		return dnsTester{}, err
	}

	containers, err := clnt.QueryContainers()
	if err != nil {
		return dnsTester{}, err
	}

	hostnameIPMap := make(map[string]string)
	for _, host := range externalHostnames {
		hostnameIPMap[host] = anyIPAllowed
	}

	for _, lb := range loadBalancers {
		hostnameIPMap[lb.Name+".q"] = lb.IP
	}

	for _, c := range containers {
		if c.Hostname != "" {
			hostnameIPMap[c.Hostname+".q"] = c.IP
		}
	}

	return dnsTester{hostnameIPMap}, nil
}

type lookupResult struct {
	hostname string
	ip       string
	err      error
	cmdTime  commandTime
}

// Resolve all hostnames on the container with the given BlueprintID. Parallelize
// over the hostnames.
func (tester dnsTester) lookupAll(container db.Container) []lookupResult {
	lookupResultsChan := make(chan lookupResult, len(tester.hostnameIPMap))

	// Create worker threads.
	lookupRequests := make(chan string, execConcurrencyLimit)
	var wg sync.WaitGroup
	for i := 0; i < execConcurrencyLimit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for hostname := range lookupRequests {
				startTime := time.Now()
				ip, err := lookup(container.BlueprintID, hostname)
				lookupResultsChan <- lookupResult{hostname, ip, err,
					commandTime{startTime, time.Now()}}
			}
		}()
	}

	// Feed worker threads.
	for hostname := range tester.hostnameIPMap {
		lookupRequests <- hostname
	}
	close(lookupRequests)
	wg.Wait()
	close(lookupResultsChan)

	// Collect results.
	var results []lookupResult
	for res := range lookupResultsChan {
		results = append(results, res)
	}

	return results
}

func (tester dnsTester) test(container db.Container) (failures []error) {
	for _, l := range tester.lookupAll(container) {
		expIP := tester.hostnameIPMap[l.hostname]
		if l.err != nil {
			failures = append(failures,
				fmt.Errorf("(%s) lookup errored: %s", l.cmdTime, l.err))
		} else if expIP != anyIPAllowed && l.ip != expIP {
			failures = append(failures,
				fmt.Errorf("(%s) expected %s, got %s",
					l.cmdTime, expIP, l.ip))
		}
	}

	return failures
}

func lookup(id string, hostname string) (string, error) {
	stdout, err := quiltSSH(id, "getent", "hosts", hostname)
	if err != nil {
		return "", err
	}

	fields := strings.Fields(stdout)
	if len(fields) < 2 {
		return "", fmt.Errorf("parse error: expected %q to have at "+
			"least 2 fields", fields)
	}

	return fields[0], nil
}

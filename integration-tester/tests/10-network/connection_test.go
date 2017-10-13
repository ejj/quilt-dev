package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/kelda/kelda/api/client"
	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/join"
)

type connectionTester struct {
	connectionMap map[string][]string
	allHostnames  []string
}

func newConnectionTester(clnt client.Client) (connectionTester, error) {
	loadBalancers, err := clnt.QueryLoadBalancers()
	if err != nil {
		return connectionTester{}, err
	}

	containers, err := clnt.QueryContainers()
	if err != nil {
		return connectionTester{}, err
	}

	connections, err := clnt.QueryConnections()
	if err != nil {
		return connectionTester{}, err
	}

	var allHostnames []string
	for _, lb := range loadBalancers {
		allHostnames = append(allHostnames, lb.Name+".q")
	}
	for _, c := range containers {
		allHostnames = append(allHostnames, c.Hostname+".q")
	}

	connectionMap := make(map[string][]string)
	for _, conn := range connections {
		connectionMap[conn.From] = append(connectionMap[conn.From], conn.To)
		// Connections are bi-directional.
		connectionMap[conn.To] = append(connectionMap[conn.To], conn.From)
	}

	return connectionTester{
		connectionMap: connectionMap,
		allHostnames:  allHostnames,
	}, nil
}

type pingResult struct {
	target    string
	reachable bool
	err       error
	cmdTime   commandTime
}

// We have to limit our parallelization because each `quilt exec` creates a new SSH login
// session. Doing this quickly in parallel breaks system-logind
// on the remote machine: https://github.com/systemd/systemd/issues/2925.
// Furthermore, the concurrency limit cannot exceed the sshd MaxStartups setting,
// or else the SSH connections may be randomly rejected.
const execConcurrencyLimit = 5

func (tester connectionTester) pingAll(container db.Container) []pingResult {
	pingResultsChan := make(chan pingResult, len(tester.allHostnames))

	// Create worker threads.
	pingRequests := make(chan string, execConcurrencyLimit)
	var wg sync.WaitGroup
	for i := 0; i < execConcurrencyLimit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for hostname := range pingRequests {
				startTime := time.Now()
				_, err := ping(container.BlueprintID, hostname)
				pingResultsChan <- pingResult{
					target:    hostname,
					reachable: err == nil,
					err:       err,
					cmdTime:   commandTime{startTime, time.Now()},
				}
			}
		}()
	}

	// Feed worker threads.
	for _, hostname := range tester.allHostnames {
		pingRequests <- hostname
	}
	close(pingRequests)
	wg.Wait()
	close(pingResultsChan)

	// Collect results.
	var pingResults []pingResult
	for res := range pingResultsChan {
		pingResults = append(pingResults, res)
	}

	return pingResults
}

func (tester connectionTester) test(container db.Container) (failures []error) {
	// We should be able to ping ourselves.
	expReachable := map[string]struct{}{
		container.Hostname + ".q": {},
	}
	for _, dst := range tester.connectionMap[container.Hostname] {
		expReachable[dst+".q"] = struct{}{}
	}

	var expPings []pingResult
	for _, ip := range tester.allHostnames {
		_, reachable := expReachable[ip]
		expPings = append(expPings, pingResult{
			target:    ip,
			reachable: reachable,
		})
	}
	pingResults := tester.pingAll(container)
	_, _, failuresIntf := join.HashJoin(pingSlice(expPings), pingSlice(pingResults),
		ignoreErrorField, ignoreErrorField)

	for _, badIntf := range failuresIntf {
		bad := badIntf.(pingResult)
		var failure error
		if bad.reachable {
			failure = fmt.Errorf("(%s) could ping unauthorized container %s",
				bad.cmdTime, bad.target)
		} else {
			failure = fmt.Errorf("(%s) couldn't ping authorized container "+
				"%s: %s", bad.cmdTime, bad.target, bad.err)
		}
		failures = append(failures, failure)
	}

	return failures
}

func ignoreErrorField(pingResultIntf interface{}) interface{} {
	return pingResult{
		target:    pingResultIntf.(pingResult).target,
		reachable: pingResultIntf.(pingResult).reachable,
	}
}

// ping `target` from within container `id` with 3 packets, with a timeout of
// 1 second for each packet.
func ping(id string, target string) (string, error) {
	return quiltSSH(id, "ping", "-c", "3", "-W", "1", target)
}

type pingSlice []pingResult

func (ps pingSlice) Get(ii int) interface{} {
	return ps[ii]
}

func (ps pingSlice) Len() int {
	return len(ps)
}

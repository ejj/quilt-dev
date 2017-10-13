package main

import (
	"bytes"
	"errors"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/integration-tester/util"
)

func TestDNS(t *testing.T) {
	clnt, err := util.GetDefaultDaemonClient()
	if err != nil {
		t.Fatalf("couldn't get api client: %s", err.Error())
	}
	defer clnt.Close()

	dnsTester, err := newDNSTester(clnt)
	if err != nil {
		t.Fatalf("couldn't initialize dns tester: %s", err.Error())
	}

	containers, err := clnt.QueryContainers()
	if err != nil {
		t.Fatalf("couldn't query containers: %s", err.Error())
	}

	// Run the test twice to see if failed tests persist.
	for i := 0; i < 2; i++ {
		testContainers(t, dnsTester, containers)
	}
}

func TestConnectivity(t *testing.T) {
	clnt, err := util.GetDefaultDaemonClient()
	if err != nil {
		t.Fatalf("couldn't get api client: %s", err.Error())
	}
	defer clnt.Close()

	connTester, err := newConnectionTester(clnt)
	if err != nil {
		t.Fatalf("couldn't initialize connection tester: %s", err.Error())
	}

	containers, err := clnt.QueryContainers()
	if err != nil {
		t.Fatalf("couldn't query containers: %s", err.Error())
	}

	// Run the test twice to see if failed tests persist.
	for i := 0; i < 2; i++ {
		testContainers(t, connTester, containers)
	}
}

type testerIntf interface {
	test(c db.Container) []error
}

type commandTime struct {
	start, end time.Time
}

func (ct commandTime) String() string {
	// Just show the hour, minute, and second.
	timeFmt := "15:04:05"
	return ct.start.Format(timeFmt) + " - " + ct.end.Format(timeFmt)
}

// Gather test results for each container. For each minion machine, run one test
// at a time.
func testContainers(t *testing.T, tester testerIntf, containers []db.Container) {
	// Create a separate test executor go routine for each minion machine.
	testChannels := make(map[string]chan db.Container)
	for _, c := range containers {
		testChannels[c.Minion] = make(chan db.Container)
	}

	var wg sync.WaitGroup
	for _, testChan := range testChannels {
		wg.Add(1)
		go func(testChan chan db.Container) {
			defer wg.Done()
			for c := range testChan {
				for _, err := range tester.test(c) {
					if err != nil {
						t.Errorf("%s: %s", c.BlueprintID, err)
					}
				}
			}
		}(testChan)
	}

	// Feed the worker threads until we've run all the tests.
	for len(containers) != 0 {
		var remainingContainers []db.Container
		for _, c := range containers {
			select {
			case testChannels[c.Minion] <- c:
			default:
				remainingContainers = append(remainingContainers, c)
			}
		}
		containers = remainingContainers
		time.Sleep(1 * time.Second)
	}
	for _, testChan := range testChannels {
		close(testChan)
	}
	wg.Wait()
}

func quiltSSH(id string, cmd ...string) (string, error) {
	execCmd := exec.Command("quilt", append([]string{"ssh", id}, cmd...)...)
	stderrBytes := bytes.NewBuffer(nil)
	execCmd.Stderr = stderrBytes

	stdoutBytes, err := execCmd.Output()
	if err != nil {
		err = errors.New(stderrBytes.String())
	}

	return string(stdoutBytes), err
}

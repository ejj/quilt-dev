package main

import (
	"os/exec"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"

	util "github.com/kelda/kelda/integration-tester/util"
)

const (
	fetcherImage      = "alpine"
	loadBalancedLabel = "loadBalanced"
)

func TestLoadBalancer(t *testing.T) {
	c, err := util.GetDefaultDaemonClient()
	if err != nil {
		t.Fatalf("couldn't get local client: %s", err)
	}
	defer c.Close()

	containers, err := c.QueryContainers()
	if err != nil {
		t.Fatalf("couldn't get containers: %s", err)
	}

	loadBalancers, err := c.QueryLoadBalancers()
	if err != nil {
		t.Fatalf("couldn't get load balancers: %s", err)
	}

	var fetcherID string
	for _, c := range containers {
		if c.Image == fetcherImage {
			fetcherID = c.BlueprintID
			break
		}
	}

	var loadBalancedContainers []string
	for _, lb := range loadBalancers {
		if lb.Name == loadBalancedLabel {
			loadBalancedContainers = lb.Hostnames
			break
		}
	}

	log.WithField("expected unique responses", len(loadBalancedContainers)).
		Info("Starting fetching..")

	if fetcherID == "" {
		t.Fatal("couldn't find fetcher")
	}

	loadBalancedCounts := map[string]int{}
	for i := 0; i < len(loadBalancedContainers)*15; i++ {
		outBytes, err := exec.Command("quilt", "ssh", fetcherID,
			"wget", "-q", "-O", "-", loadBalancedLabel+".q").
			CombinedOutput()
		if err != nil {
			t.Errorf("Unable to GET: %s", err)
			continue
		}

		loadBalancedCounts[strings.TrimSpace(string(outBytes))]++
	}

	log.WithField("counts", loadBalancedCounts).Info("Fetching completed")
	if len(loadBalancedCounts) < len(loadBalancedContainers) {
		t.Fatalf("some containers not load balanced: "+
			"expected to query %d containers, got %d",
			len(loadBalancedContainers), len(loadBalancedCounts))
	}
}

func contains(lst []string, key string) bool {
	for _, v := range lst {
		if v == key {
			return true
		}
	}
	return false
}

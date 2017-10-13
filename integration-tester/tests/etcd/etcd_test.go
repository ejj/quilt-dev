package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/integration-tester/util"
)

func TestEtcd(t *testing.T) {
	clnt, err := util.GetDefaultDaemonClient()
	if err != nil {
		t.Fatalf("couldn't get api client: %s", err)
	}
	defer clnt.Close()

	containers, err := clnt.QueryContainers()
	if err != nil {
		t.Fatalf("couldn't query containers: %s", err)
	}

	test(t, containers)
}

func test(t *testing.T, containers []db.Container) {
	for _, c := range containers {
		if !strings.Contains(c.Image, "etcd") {
			continue
		}

		fmt.Printf("Checking etcd health from %s\n", c.BlueprintID)
		out, err := exec.Command("quilt", "ssh", c.BlueprintID,
			"etcdctl", "cluster-health").CombinedOutput()
		fmt.Println(string(out))
		if err != nil || !strings.Contains(string(out), "cluster is healthy") {
			t.Errorf("cluster is unhealthy: %s", err)
		}
	}
}

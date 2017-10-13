package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/integration-tester/util"

	"github.com/satori/go.uuid"
)

func TestZookeeper(t *testing.T) {
	clnt, err := util.GetDefaultDaemonClient()
	if err != nil {
		t.Fatalf("couldn't get api client: %s", err)
	}
	defer clnt.Close()

	containers, err := clnt.QueryContainers()
	if err != nil {
		t.Fatalf("couldn't query containers: %s", err)
	}

	var zkContainers []db.Container
	for _, c := range containers {
		if strings.Contains(c.Image, "zookeeper") {
			zkContainers = append(zkContainers, c)
		}
	}

	test(t, zkContainers)
}

// Write a random key value pair to each zookeeper node, and then ensure that
// all nodes can retrieve all the written keys.
func test(t *testing.T, containers []db.Container) {
	expData := map[string]string{}
	for _, c := range containers {
		key := "/" + uuid.NewV4().String()
		expData[key] = uuid.NewV4().String()

		fmt.Printf("Writing %s to key %s from %s\n",
			expData[key], key, c.BlueprintID)
		out, err := exec.Command("quilt", "ssh", c.BlueprintID,
			"bin/zkCli.sh", "create", key, expData[key]).CombinedOutput()
		if err != nil {
			t.Errorf("unable to create key: %s", err)
			fmt.Println(string(out))
		}
	}

	for _, c := range containers {
		for key, val := range expData {
			fmt.Printf("Getting key %s from %s: expect %s\n",
				key, c.BlueprintID, val)
			out, err := exec.Command("quilt", "ssh", c.BlueprintID,
				"bin/zkCli.sh", "get", key).CombinedOutput()
			if err != nil || !strings.Contains(string(out), val) {
				t.Errorf("unexpected value: %s", err)
				fmt.Println(string(out))
			}
		}
	}
}

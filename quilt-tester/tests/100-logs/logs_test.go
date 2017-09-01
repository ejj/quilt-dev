package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/quilt/quilt/api"
	"github.com/quilt/quilt/api/client"
	"github.com/quilt/quilt/connection/credentials"
)

func TestMinionLogs(t *testing.T) {
	if err := printQuiltPs(); err != nil {
		t.Errorf("failed to print quilt ps: %s", err.Error())
	}

	c, err := client.New(api.DefaultSocket, credentials.Insecure{})
	if err != nil {
		t.Fatalf("couldn't get quiltctl client: %s", err.Error())
	}
	defer c.Close()

	machines, err := c.QueryMachines()
	if err != nil {
		t.Fatalf("couldn't query the machines: %s", err.Error())
	}

	for _, machine := range machines {
		fmt.Println(machine)
		logsOutput, err := exec.Command("quilt", "ssh", machine.StitchID,
			"sudo", "journalctl", "-o", "cat", "-u", "minion").
			CombinedOutput()
		if err != nil {
			t.Errorf("unable to get minion logs: %s", err.Error())
			continue
		}
		outputStr := string(logsOutput)
		fmt.Println(outputStr)
		checkString(t, outputStr)
	}
}

func checkString(t *testing.T, str string) {
	for _, line := range strings.Split(str, "\n") {
		// Errors pulling an image are completely out of our control.  They
		// should still be logged as warnings to the user, but they shouldn't
		// cause a test failure.
		if strings.Contains(line, "pull image error") {
			fmt.Printf("Ignoring pull image error: %s\n", line)
			continue
		}

		// "goroutine 0" is the main goroutine and is printed in stacktraces.
		if strings.Contains(line, "goroutine 0") {
			t.Error("Minion logs has a stack trace")
		}

		// The trailing open bracket is necessary to filter out false positives
		// in the log output. For example, as part of logging DNS requests, the
		// status string NOERROR is printed.
		if strings.Contains(line, "ERROR [") ||
			strings.Contains(line, "WARN [") {
			t.Errorf("Minion logs has error: %s", line)
		}
	}
}

func printQuiltPs() error {
	psout, err := exec.Command("quilt", "ps").CombinedOutput()
	fmt.Println(string(psout))
	return err
}

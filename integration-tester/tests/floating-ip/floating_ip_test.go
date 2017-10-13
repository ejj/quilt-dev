package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/integration-tester/util"
)

// This map should match the map in floating-ip.js.
var providerToFloatingIP = map[db.ProviderName]string{
	db.Amazon:       "13.57.99.49",
	db.Google:       "104.196.11.66",
	db.DigitalOcean: "138.68.203.188",
}

func TestFloatingIP(t *testing.T) {
	clnt, err := util.GetDefaultDaemonClient()
	if err != nil {
		t.Fatalf("couldn't get api client: %s", err)
	}
	defer clnt.Close()

	machines, err := clnt.QueryMachines()
	if err != nil || len(machines) == 0 {
		t.Fatalf("couldn't query machines: %s", err)
	}

	provider := machines[0].Provider
	floatingIP, ok := providerToFloatingIP[provider]
	if !ok {
		t.Fatalf("no floating IP for provider %s", floatingIP)
	}

	url := "http://" + floatingIP
	fmt.Printf("Querying %s\n", url)
	resp, err := http.Get(url)
	fmt.Printf("Got: %v\n", resp)
	if err != nil {
		t.Fatalf("unable to retrieve %s: %s", url, err)
	}
}

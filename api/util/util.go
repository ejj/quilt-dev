package util

import (
	"fmt"

	"github.com/kelda/kelda/db"
)

// GetContainer retrieves the container with the given blueprintID.
func GetContainer(containers []db.Container, blueprintID string) (db.Container, error) {
	var choice *db.Container
	for _, c := range containers {
		if len(blueprintID) > len(c.BlueprintID) ||
			c.BlueprintID[0:len(blueprintID)] != blueprintID {
			continue
		}

		if choice != nil {
			err := fmt.Errorf("ambiguous blueprintIDs %s and %s",
				choice.BlueprintID, c.BlueprintID)
			return db.Container{}, err
		}

		copy := c
		choice = &copy
	}

	if choice != nil {
		return *choice, nil
	}

	err := fmt.Errorf("no container with blueprintID %q", blueprintID)
	return db.Container{}, err
}

// GetPublicIP returns the public IP for the machine with the given private IP.
func GetPublicIP(machines []db.Machine, privateIP string) (string, error) {
	for _, m := range machines {
		if m.PrivateIP == privateIP {
			return m.PublicIP, nil
		}
	}

	return "", fmt.Errorf("no machine with private IP %s", privateIP)
}

package engine

import (
	"testing"

	"github.com/kelda/kelda/blueprint"
	"github.com/kelda/kelda/db"
	"github.com/stretchr/testify/assert"
)

func TestEngine(t *testing.T) {
	conn := db.New()

	bp := blueprint.Blueprint{
		Namespace: "namespace",
		Machines: []blueprint.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "2"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "3"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "4"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "5"},
		},
	}
	updateBlueprint(t, conn, bp, "")

	masters, workers := selectMachines(conn)
	assert.Equal(t, 2, len(masters))
	assert.Equal(t, 3, len(workers))

	/* Verify master increase. */
	bp.Machines = append(bp.Machines,
		blueprint.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Master", ID: "6"},
		blueprint.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Master", ID: "7"},
		blueprint.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Worker", ID: "8"},
		blueprint.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Worker", ID: "9"},
	)

	updateBlueprint(t, conn, bp, "")
	masters, workers = selectMachines(conn)
	assert.Equal(t, 4, len(masters))
	assert.Equal(t, 5, len(workers))

	/* Verify that external writes stick around. */
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		for _, master := range masters {
			master.CloudID = "1"
			master.PublicIP = "2"
			master.PrivateIP = "3"
			view.Commit(master)
		}

		for _, worker := range workers {
			worker.CloudID = "1"
			worker.PublicIP = "2"
			worker.PrivateIP = "3"
			view.Commit(worker)
		}

		return nil
	})

	/* Also verify that masters and workers decrease properly. */
	bp.Machines = []blueprint.Machine{
		{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
		{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "3"},
	}
	updateBlueprint(t, conn, bp, "")

	masters, workers = selectMachines(conn)

	assert.Equal(t, 1, len(masters))
	assert.Equal(t, "1", masters[0].CloudID)
	assert.Equal(t, "2", masters[0].PublicIP)
	assert.Equal(t, "3", masters[0].PrivateIP)

	assert.Equal(t, 1, len(workers))
	assert.Equal(t, "1", workers[0].CloudID)
	assert.Equal(t, "2", workers[0].PublicIP)
	assert.Equal(t, "3", workers[0].PrivateIP)

	/* Empty Namespace does nothing. */
	bp.Namespace = ""
	updateBlueprint(t, conn, bp, "")
	masters, workers = selectMachines(conn)

	assert.Equal(t, 1, len(masters))
	assert.Equal(t, "1", masters[0].CloudID)
	assert.Equal(t, "2", masters[0].PublicIP)
	assert.Equal(t, "3", masters[0].PrivateIP)

	assert.Equal(t, 1, len(workers))
	assert.Equal(t, "1", workers[0].CloudID)
	assert.Equal(t, "2", workers[0].PublicIP)
	assert.Equal(t, "3", workers[0].PrivateIP)

	/* Verify things go to zero. */
	updateBlueprint(t, conn, blueprint.Blueprint{
		Machines: []blueprint.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Worker"},
		},
	}, "")
	masters, workers = selectMachines(conn)
	assert.Zero(t, len(masters))
	assert.Zero(t, len(workers))

	// This function checks whether there is a one-to-one mapping for each machine
	// in `slice` to a provider in `providers`.
	assertProvidersInSlice := func(
		slice db.MachineSlice, providers []db.ProviderName) {
		for _, p := range providers {
			found := false
			for _, m := range slice {
				if m.Provider == p {
					found = true
					break
				}
			}
			assert.True(t, found)
		}
		// Make sure there are no extra machines.
		assert.Equal(t, len(slice), len(providers))
	}

	/* Test mixed providers. */
	updateBlueprint(t, conn, blueprint.Blueprint{
		Machines: []blueprint.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
			{Provider: "Vagrant", Size: "v.large", Role: "Master", ID: "2"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "3"},
			{Provider: "Google", Size: "g.large", Role: "Worker", ID: "4"},
		},
	}, "")
	masters, workers = selectMachines(conn)
	assertProvidersInSlice(masters, []db.ProviderName{db.Amazon, db.Vagrant})
	assertProvidersInSlice(workers, []db.ProviderName{db.Amazon, db.Google})

	/* Test that machines with different providers don't match. */
	updateBlueprint(t, conn, blueprint.Blueprint{
		Machines: []blueprint.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "2"},
		},
	}, "")
	masters, _ = selectMachines(conn)
	assertProvidersInSlice(masters, []db.ProviderName{db.Amazon})
}

func TestAdminKey(t *testing.T) {
	t.Parallel()

	conn := db.New()

	updateBlueprint(t, conn, blueprint.Blueprint{
		Machines: []blueprint.Machine{
			{
				ID:       "1",
				Provider: "Amazon",
				Role:     "Master",
				SSHKeys:  []string{"app"},
			},
			{
				ID:       "2",
				Provider: "Amazon",
				Role:     "Worker",
				SSHKeys:  []string{"app"},
			},
		},
	}, "admin")

	machines := conn.SelectFromMachine(nil)
	assert.Len(t, machines, 2)
	for _, m := range machines {
		assert.Equal(t, []string{"app", "admin"}, m.SSHKeys)
	}

	updateBlueprint(t, conn, blueprint.Blueprint{
		Machines: []blueprint.Machine{
			{
				ID:       "1",
				Provider: "Amazon",
				Role:     "Master",
				SSHKeys:  []string{"app"},
			},
			{
				ID:       "2",
				Provider: "Amazon",
				Role:     "Worker",
				SSHKeys:  []string{"app"},
			},
		},
	}, "")

	machines = conn.SelectFromMachine(nil)
	assert.Len(t, machines, 2)
	for _, m := range machines {
		assert.Equal(t, []string{"app"}, m.SSHKeys)
	}
}

func TestSort(t *testing.T) {
	conn := db.New()

	updateBlueprint(t, conn, blueprint.Blueprint{
		Machines: []blueprint.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master"},
			{Provider: "Amazon", Size: "m4.large", Role: "Master"},
			{Provider: "Amazon", Size: "m4.large", Role: "Master"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker"},
		},
	}, "")
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		assert.Equal(t, 3, len(machines))

		machines[0].BlueprintID = ""
		view.Commit(machines[0])

		machines[2].BlueprintID = ""
		machines[2].PublicIP = "a"
		machines[2].PrivateIP = "b"
		view.Commit(machines[2])

		machines[1].BlueprintID = ""
		machines[1].PrivateIP = "c"
		view.Commit(machines[1])

		return nil
	})

	updateBlueprint(t, conn, blueprint.Blueprint{
		Machines: []blueprint.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master"},
			{Provider: "Amazon", Size: "m4.large", Role: "Master"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker"},
		},
	}, "")
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		assert.Equal(t, 2, len(machines))

		for _, m := range machines {
			assert.False(t, m.PublicIP == "" && m.PrivateIP == "")
		}

		return nil
	})
}

func selectMachines(conn db.Conn) (masters, workers []db.Machine) {
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		masters = view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers = view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})
		return nil
	})
	return
}

func updateBlueprint(t *testing.T, conn db.Conn, newBlueprint blueprint.Blueprint,
	adminKey string) {
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		bp, err := view.GetBlueprint()
		if err != nil {
			bp = view.InsertBlueprint()
		}
		bp.Blueprint = newBlueprint
		view.Commit(bp)
		return nil
	})
	assert.Nil(t, conn.Txn(db.AllTables...).Run(
		func(view db.Database) error {
			return updateTxn(view, adminKey)
		}))
}

package engine

import (
	"testing"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/join"
	"github.com/quilt/quilt/stitch"
	"github.com/stretchr/testify/assert"
)

func TestEngine(t *testing.T) {
	conn := db.New()

	stc := stitch.Stitch{
		Namespace: "namespace",
		Machines: []stitch.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "2"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "3"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "4"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "5"},
		},
	}
	updateStitch(t, conn, stc, "")

	masters, workers := selectMachines(conn)
	assert.Equal(t, 2, len(masters))
	assert.Equal(t, 3, len(workers))

	/* Verify master increase. */
	stc.Machines = append(stc.Machines,
		stitch.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Master", ID: "6"},
		stitch.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Master", ID: "7"},
		stitch.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Worker", ID: "8"},
		stitch.Machine{Provider: "Amazon", Size: "m4.large",
			Role: "Worker", ID: "9"},
	)

	updateStitch(t, conn, stc, "")
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
	stc.Machines = []stitch.Machine{
		{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
		{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "3"},
	}
	updateStitch(t, conn, stc, "")

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
	stc.Namespace = ""
	updateStitch(t, conn, stc, "")
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
	updateStitch(t, conn, stitch.Stitch{
		Machines: []stitch.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Worker"},
		},
	}, "")
	masters, workers = selectMachines(conn)
	assert.Zero(t, len(masters))
	assert.Zero(t, len(workers))

	// This function checks whether there is a one-to-one mapping for each machine
	// in `slice` to a provider in `providers`.
	providersInSlice := func(slice db.MachineSlice, providers db.ProviderSlice) bool {
		lKey := func(left interface{}) interface{} {
			return left.(db.Machine).Provider
		}
		rKey := func(right interface{}) interface{} {
			return right.(db.Provider)
		}
		_, l, r := join.HashJoin(slice, providers, lKey, rKey)
		return len(l) == 0 && len(r) == 0
	}

	/* Test mixed providers. */
	updateStitch(t, conn, stitch.Stitch{
		Machines: []stitch.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
			{Provider: "Vagrant", Size: "v.large", Role: "Master", ID: "2"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "3"},
			{Provider: "Google", Size: "g.large", Role: "Worker", ID: "4"},
		},
	}, "")
	masters, workers = selectMachines(conn)
	assert.True(t, providersInSlice(masters,
		db.ProviderSlice{db.Amazon, db.Vagrant}))
	assert.True(t, providersInSlice(workers, db.ProviderSlice{db.Amazon, db.Google}))

	/* Test that machines with different providers don't match. */
	updateStitch(t, conn, stitch.Stitch{
		Machines: []stitch.Machine{
			{Provider: "Amazon", Size: "m4.large", Role: "Master", ID: "1"},
			{Provider: "Amazon", Size: "m4.large", Role: "Worker", ID: "2"},
		},
	}, "")
	masters, _ = selectMachines(conn)
	assert.True(t, providersInSlice(masters, db.ProviderSlice{db.Amazon}))
}

func TestAdminKey(t *testing.T) {
	t.Parallel()

	conn := db.New()

	updateStitch(t, conn, stitch.Stitch{
		Machines: []stitch.Machine{
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

	updateStitch(t, conn, stitch.Stitch{
		Machines: []stitch.Machine{
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

	updateStitch(t, conn, stitch.Stitch{
		Machines: []stitch.Machine{
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

		machines[0].StitchID = ""
		view.Commit(machines[0])

		machines[2].StitchID = ""
		machines[2].PublicIP = "a"
		machines[2].PrivateIP = "b"
		view.Commit(machines[2])

		machines[1].StitchID = ""
		machines[1].PrivateIP = "c"
		view.Commit(machines[1])

		return nil
	})

	updateStitch(t, conn, stitch.Stitch{
		Machines: []stitch.Machine{
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

func updateStitch(t *testing.T, conn db.Conn, stitch stitch.Stitch, adminKey string) {
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		blueprint, err := view.GetBlueprint()
		if err != nil {
			blueprint = view.InsertBlueprint()
		}
		blueprint.Stitch = stitch
		view.Commit(blueprint)
		return nil
	})
	assert.Nil(t, conn.Txn(db.AllTables...).Run(
		func(view db.Database) error {
			return updateTxn(view, adminKey)
		}))
}

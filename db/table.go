package db

import (
	"reflect"
	"sync"
)

// TableType represents a table in the database.
type TableType string

// BlueprintTable is the type of the blueprint table.
var BlueprintTable = TableType(reflect.TypeOf(Blueprint{}).String())

// MachineTable is the type of the machine table.
var MachineTable = TableType(reflect.TypeOf(Machine{}).String())

// ContainerTable is the type of the container table.
var ContainerTable = TableType(reflect.TypeOf(Container{}).String())

// MinionTable is the type of the minion table.
var MinionTable = TableType(reflect.TypeOf(Minion{}).String())

// ConnectionTable is the type of the connection table.
var ConnectionTable = TableType(reflect.TypeOf(Connection{}).String())

// LabelTable is the type of the label table.
var LabelTable = TableType(reflect.TypeOf(Label{}).String())

// EtcdTable is the type of the etcd table.
var EtcdTable = TableType(reflect.TypeOf(Etcd{}).String())

// PlacementTable is the type of the placement table.
var PlacementTable = TableType(reflect.TypeOf(Placement{}).String())

// ACLTable is the type of the ACL table.
var ACLTable = TableType(reflect.TypeOf(ACL{}).String())

// ImageTable is the type of the image table.
var ImageTable = TableType(reflect.TypeOf(Image{}).String())

// HostnameTable is the type of the Hostname table.
var HostnameTable = TableType(reflect.TypeOf(Hostname{}).String())

// AllTables is a slice of all the db TableTypes. It is used primarily for tests,
// where there is no reason to put lots of thought into which tables a Transaction
// should use.
var AllTables = []TableType{BlueprintTable, MachineTable, ContainerTable, MinionTable,
	ConnectionTable, LabelTable, EtcdTable, PlacementTable, ACLTable, ImageTable,
	HostnameTable}

type table struct {
	rows map[int]row

	triggers    map[Trigger]struct{}
	shouldAlert bool
	sync.Mutex
}

func newTable() *table {
	return &table{
		rows:        make(map[int]row),
		triggers:    make(map[Trigger]struct{}),
		shouldAlert: false,
	}
}

func (t *table) alert() {
	for trigger := range t.triggers {
		select {
		case <-trigger.stop:
			delete(t.triggers, trigger)
			continue
		default:
		}

		select {
		case trigger.C <- struct{}{}:
			c.Inc("Trigger")
		default:
		}
	}
}

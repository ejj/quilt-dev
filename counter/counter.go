package counter

import (
	"sync"
	"sync/atomic"

	"github.com/quilt/quilt/api/pb"
	"golang.org/x/sync/syncmap"
)

// Package contains a collection of counters under a single name.
type Package struct {
	name string
}

// XXX: Note syncmap.Map is a prototype that will be upstreamed into the go standard
// library on the next release.  At that time, we should switch to the standard library
// version.
var all = syncmap.Map{}

// New creates a new Package with the given Name.
func New(name string) Package {
	return Package{name}
}

// Inc increments the counter `name` under the provided package.
func (p Package) Inc(name string) {
	key := struct{ p, n string }{p.name, name}
	c, _ := all.LoadOrStore(key, &pb.Counter{Pkg: p.name, Name: name})
	atomic.AddUint64(&c.(*pb.Counter).Value, 1)
}

var dumpMutex = sync.Mutex{}

// Dump returns a list of all in no particular order.
func Dump() []*pb.Counter {
	var result []*pb.Counter
	dumpMutex.Lock()
	all.Range(func(key, value interface{}) bool {
		counter := value.(*pb.Counter)
		cpy := *counter

		// This is not thread-safe if any other code in this module can update
		// PrevValue.  It's only thread-safe here because of the dumpMutex.
		val := atomic.LoadUint64(&counter.Value)
		atomic.StoreUint64(&counter.PrevValue, val)

		result = append(result, &cpy)
		return true
	})
	dumpMutex.Unlock()
	return result
}

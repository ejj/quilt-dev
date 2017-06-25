package counter

import (
	"sync"
	"testing"

	"github.com/quilt/quilt/api/pb"
	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	var c1 = New("a")
	var c2 = New("b")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < 10000; j++ {
				c1.Inc("1")
				c1.Inc("2")
				c2.Inc("1")
			}
			wg.Done()
		}()
	}
	wg.Wait()

	res := Dump()
	assert.Len(t, res, 3)
	assert.Contains(t, res, &pb.Counter{Pkg: "a", Name: "1", Value: 1000000})
	assert.Contains(t, res, &pb.Counter{Pkg: "a", Name: "2", Value: 1000000})
	assert.Contains(t, res, &pb.Counter{Pkg: "b", Name: "1", Value: 1000000})

	wg = sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < 100; j++ {
				c1.Inc("1")
				c2.Inc("1")
			}
			wg.Done()
		}()
	}
	wg.Wait()

	res = Dump()
	assert.Len(t, res, 3)
	assert.Contains(t, res, &pb.Counter{
		Pkg: "a", Name: "1", Value: 1001000, PrevValue: 1000000})
	assert.Contains(t, res, &pb.Counter{
		Pkg: "a", Name: "2", Value: 1000000, PrevValue: 1000000})
	assert.Contains(t, res, &pb.Counter{
		Pkg: "b", Name: "1", Value: 1001000, PrevValue: 1000000})

	res = Dump()
	assert.Len(t, res, 3)
	assert.Contains(t, res, &pb.Counter{
		Pkg: "a", Name: "1", Value: 1001000, PrevValue: 1001000})
	assert.Contains(t, res, &pb.Counter{
		Pkg: "a", Name: "2", Value: 1000000, PrevValue: 1000000})
	assert.Contains(t, res, &pb.Counter{
		Pkg: "b", Name: "1", Value: 1001000, PrevValue: 1001000})
}

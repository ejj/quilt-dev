package command

import (
	"bytes"
	"flag"
	"fmt"
	"testing"

	"github.com/quilt/quilt/api/client/mocks"
	"github.com/quilt/quilt/api/pb"
	"github.com/stretchr/testify/assert"
)

func TestCounterFlags(t *testing.T) {
	t.Parallel()

	counters := &Counters{}

	flags := &flag.FlagSet{}
	assert.Nil(t, flags.Usage)

	counters.InstallFlags(flags)

	assert.NotNil(t, flags.Usage)
}

func TestParse(t *testing.T) {
	t.Parallel()

	counters := &Counters{}
	assert.Nil(t, counters.Parse(nil))
}

func TestRun(t *testing.T) {
	t.Parallel()

	counters := &Counters{}
	mock := new(mocks.Client)
	counters.client = mock

	mock.On("QueryCounters").Once().Return(nil, assert.AnError)
	assert.NotZero(t, counters.Run())

	mock.On("QueryCounters").Once().Return(nil, nil)
	assert.Zero(t, counters.Run())
}

func TestPrintCounters(t *testing.T) {
	t.Parallel()

	counters := []pb.Counter{{
		Pkg:       "PkgA",
		Name:      "NameA",
		Value:     100,
		PrevValue: 56,
	}, {
		Pkg:       "PkgA",
		Name:      "NameBBBBBBBBBBBBBBB",
		Value:     200,
		PrevValue: 200,
	}, {
		Pkg:       "PkgB",
		Name:      "C",
		Value:     300,
		PrevValue: 0}}

	var b bytes.Buffer
	printCounters(&b, counters)
	fmt.Println(b.String())
	assert.Equal(t, `COUNTER                  VALUE  DELTA
                                
PkgA                              
    NameA                100    44
    NameBBBBBBBBBBBBBBB  200    0
PkgB                              
    C                    300    300
`, b.String())
}

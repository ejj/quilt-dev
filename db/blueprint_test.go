package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlueprint(t *testing.T) {
	conn := New()

	ns, err := conn.GetBlueprintNamespace()
	assert.NotNil(t, err)
	assert.Exactly(t, ns, "")

	conn.Txn(AllTables...).Run(func(view Database) error {
		bp := view.InsertBlueprint()
		bp.Namespace = "test"
		view.Commit(bp)
		return nil
	})

	ns, err = conn.GetBlueprintNamespace()
	assert.NoError(t, err)
	assert.Exactly(t, ns, "test")

	bps := conn.SelectFromBlueprint(nil)
	assert.Len(t, bps, 1)

	assert.Equal(t, BlueprintTable, bps[0].tt())
	assert.True(t, bps[0].less(Blueprint{ID: bps[0].ID + 1}))

	assert.Equal(t, "Blueprint-1{Stitch={\"Namespace\":\"test\"}}", bps[0].String())
}

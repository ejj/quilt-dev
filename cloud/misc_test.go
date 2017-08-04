package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quilt/quilt/db"
)

func TestDefaultRegion(t *testing.T) {
	exp := "foo"
	m := db.Machine{Provider: "Amazon", Region: exp}
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m = DefaultRegion(m)
	exp = "us-west-1"
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m.Provider = "DigitalOcean"
	exp = "sfo1"
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m.Provider = "Google"
	exp = "us-east1-b"
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m.Provider = "Vagrant"
	exp = ""
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m.Provider = "Panic"
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic")
		}
	}()

	m = DefaultRegion(m)
}

func TestNewProviderFailure(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("provider.New did not panic on invalid provider")
		}
	}()
	newProviderImpl("FakeAmazon", testRegion, "namespace")
}

func TestGroupBy(t *testing.T) {
	t.Parallel()

	grouped := groupByLoc([]joinMachine{
		{provider: db.Google}, {provider: db.Amazon}, {provider: db.Google},
		{provider: db.Google},
	})
	assert.Equal(t, map[launchLoc][]db.Machine{
		{db.Amazon, ""}: {{}},
		{db.Google, ""}: {{}, {}, {}},
	}, grouped)
}

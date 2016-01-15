package cloudcfg

import (
	"testing"

	"github.com/quilt/quilt/db"

	log "github.com/Sirupsen/logrus"
)

func TestCloudConfig(t *testing.T) {
	cfgTemplate = "({{.QuiltImage}}) ({{.SSHKeys}}) ({{.UbuntuVersion}}) " +
		"({{.MinionOpts}}) ({{.LogLevel}})"

	log.SetLevel(log.InfoLevel)
	ver = "master"
	res := Ubuntu(Options{
		SSHKeys:    []string{"a", "b"},
		MinionOpts: MinionOptions{Role: db.Master},
	})
	exp := "(ethanj/quilt) (a\nb) (xenial) (--role \"Master\") (info)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}

	log.SetLevel(log.DebugLevel)
	ver = "1.2.3"
	res = Ubuntu(Options{
		SSHKeys:    []string{"a", "b"},
		MinionOpts: MinionOptions{Role: db.Worker},
	})
	exp = "(ethanj/quilt) (a\nb) (xenial) (--role \"Worker\") (debug)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}
}

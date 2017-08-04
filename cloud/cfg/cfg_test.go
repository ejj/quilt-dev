package cfg

import (
	"testing"

	"github.com/quilt/quilt/db"

	log "github.com/Sirupsen/logrus"
)

func TestCloudConfig(t *testing.T) {
	cfgTemplate = "({{.QuiltImage}}) ({{.SSHKeys}}) " +
		"({{.MinionOpts}}) ({{.LogLevel}}) ({{.DockerOpts}})"

	log.SetLevel(log.InfoLevel)
	ver = "master"
	res := Ubuntu(db.Machine{
		SSHKeys: []string{"a", "b"},
		Role:    db.Master,
	}, "")
	exp := "(quilt/quilt:master) (a\nb) (--role \"Master\") (info) ()"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}

	log.SetLevel(log.DebugLevel)
	ver = "1.2.3"
	MinionTLSDir = "dir"
	res = Ubuntu(db.Machine{
		SSHKeys: []string{"a", "b"},
		Role:    db.Worker,
	}, "ib")
	exp = "(quilt/quilt:1.2.3) (a\nb) (--role \"Worker\" --inbound-pub-intf \"ib\"" +
		" --tls-dir \"dir\") (debug) (-v dir:dir:ro)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}
}

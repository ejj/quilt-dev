package cfg

import (
	"testing"

	"github.com/kelda/kelda/db"

	log "github.com/sirupsen/logrus"
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
	exp := "(quilt/quilt:master) (a\nb) (--role \"Master\") (info)" +
		" (-v /home/quilt/.quilt/tls:/home/quilt/.quilt/tls:ro)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}

	log.SetLevel(log.DebugLevel)
	ver = "1.2.3"
	res = Ubuntu(db.Machine{
		SSHKeys: []string{"a", "b"},
		Role:    db.Worker,
	}, "ib")
	exp = "(quilt/quilt:1.2.3) (a\nb) (--role \"Worker\"" +
		" --inbound-pub-intf \"ib\") (debug)" +
		" (-v /home/quilt/.quilt/tls:/home/quilt/.quilt/tls:ro)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}
}

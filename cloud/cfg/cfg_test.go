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
	res := Ubuntu(Options{
		SSHKeys:    []string{"a", "b"},
		MinionOpts: MinionOptions{Role: db.Master},
	})
	exp := "(quilt/quilt:master) (a\nb) (--role \"Master\") (info) ()"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}

	log.SetLevel(log.DebugLevel)
	ver = "1.2.3"
	res = Ubuntu(Options{
		SSHKeys:    []string{"a", "b"},
		MinionOpts: MinionOptions{Role: db.Worker, TLSDir: "dir"},
	})
	exp = "(quilt/quilt:1.2.3) (a\nb) (--role \"Worker\" " +
		"--tls-dir \"dir\") (debug) (-v dir:dir:ro)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}
}

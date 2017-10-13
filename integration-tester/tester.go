package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/kelda/kelda/blueprint"
	cliPath "github.com/kelda/kelda/cli/path"
	"github.com/kelda/kelda/db"
	testerUtil "github.com/kelda/kelda/integration-tester/util"
	"github.com/kelda/kelda/join"
	"github.com/kelda/kelda/util"
)

var (
	infrastructureBlueprint = "./config/infrastructureRunner.js"
)

// The global logger for this CI run.
var log logger

func main() {
	namespace := os.Getenv("TESTING_NAMESPACE")
	if namespace == "" {
		logrus.Error("Please set TESTING_NAMESPACE.")
		os.Exit(1)
	}

	var err error
	if log, err = newLogger(); err != nil {
		logrus.WithError(err).Error("Failed to create logger.")
		os.Exit(1)
	}

	tester, err := newTester(namespace)
	if err != nil {
		logrus.WithError(err).Error("Failed to create tester instance.")
		os.Exit(1)
	}

	if err := tester.run(); err != nil {
		logrus.WithError(err).Error("Test execution failed.")
		os.Exit(1)
	}
}

type tester struct {
	preserveFailed bool
	junitOut       string

	testSuites  []*testSuite
	initialized bool
	namespace   string
}

func newTester(namespace string) (tester, error) {
	t := tester{
		namespace: namespace,
	}

	testRoot := flag.String("testRoot", "",
		"the root directory containing the integration tests")
	flag.BoolVar(&t.preserveFailed, "preserve-failed", false,
		"don't destroy machines on failed tests")
	flag.StringVar(&t.junitOut, "junitOut", "",
		"location to write junit report")
	flag.Parse()

	if *testRoot == "" {
		return tester{}, errors.New("testRoot is required")
	}

	err := t.generateTestSuites(*testRoot)
	if err != nil {
		return tester{}, err
	}

	return t, nil
}

func (t *tester) generateTestSuites(testRoot string) error {
	l := log.testerLogger

	// First, we need to ls the testRoot, and find all of the folders. Then we can
	// generate a testSuite for each folder.
	testSuiteFolders, err := filepath.Glob(filepath.Join(testRoot, "*"))
	if err != nil {
		l.infoln("Could not access test suite folders")
		l.errorln(err.Error())
		return err
	}

	sort.Sort(byPriorityPrefix(testSuiteFolders))
	for _, testSuiteFolder := range testSuiteFolders {
		files, err := ioutil.ReadDir(testSuiteFolder)
		if err != nil {
			l.infoln(fmt.Sprintf(
				"Error reading test suite %s", testSuiteFolder))
			l.errorln(err.Error())
			return err
		}

		var blueprint, test string
		for _, file := range files {
			path := filepath.Join(testSuiteFolder, file.Name())
			switch {
			case strings.HasSuffix(file.Name(), ".js"):
				blueprint = path
				err := updateNamespace(blueprint, t.namespace)
				if err != nil {
					l.infoln(fmt.Sprintf(
						"Error updating namespace for %s.",
						blueprint))
					l.errorln(err.Error())
					return err
				}
			// If the file is executable by everyone, and is not a directory.
			case (file.Mode()&1 != 0) && !file.IsDir():
				test = path
			}
		}
		newSuite := testSuite{
			name:      filepath.Base(testSuiteFolder),
			blueprint: "./" + blueprint,
			test:      test,
		}
		t.testSuites = append(t.testSuites, &newSuite)
	}

	return nil
}

func (t tester) run() error {
	defer func() {
		junitReport := newJUnitReport(t.testSuites)
		if t.junitOut != "" {
			writeJUnitReport(t.junitOut, junitReport)
		}

		failed := false
		for _, result := range junitReport.TestResults {
			if result.Failure != nil {
				failed = true
				break
			}
		}

		if failed && t.preserveFailed {
			return
		}

		stop(t.namespace)
	}()

	if err := t.setup(); err != nil {
		log.testerLogger.errorln("Unable to setup the tests, bailing.")
		// All suites failed if we didn't run them.
		for _, suite := range t.testSuites {
			suite.passed = false
		}
		return err
	}

	return t.runTestSuites()
}

func (t *tester) setup() error {
	l := log.testerLogger

	l.infoln("Starting the Quilt daemon.")
	go runQuiltDaemon()

	// Get blueprint dependencies.
	l.infoln("Installing blueprint dependencies")
	_, _, err := npmInstall()
	if err != nil {
		l.infoln("Could not install dependencies")
		l.errorln(err.Error())
		return err
	}

	// Wait for the daemon to generate the TLS credentials. If we don't wait,
	// the subsequent Quilt commands (such as `quilt stop`) might fail if they
	// are executed before the credentials are generated.
	err = util.BackoffWaitFor(func() bool {
		_, err = util.Stat(cliPath.DefaultTLSDir)
		return err == nil
	}, 15*time.Second, 3*time.Minute)
	if err != nil {
		l.infoln("Timed out waiting for daemon to generate TLS credentials")
		return err
	}

	// Do a preliminary quilt stop.
	l.infoln(fmt.Sprintf("Preliminary `quilt stop %s`", t.namespace))
	_, _, err = stop(t.namespace)
	if err != nil {
		l.infoln(fmt.Sprintf("Error stopping: %s", err.Error()))
		return err
	}

	// Setup infrastructure.
	l.infoln("Booting the machines the test suites will run on, and waiting " +
		"for them to connect back.")
	l.infoln("Begin " + infrastructureBlueprint)
	if err := updateNamespace(infrastructureBlueprint, t.namespace); err != nil {
		l.infoln(fmt.Sprintf("Error updating namespace for %s.",
			infrastructureBlueprint))
		l.errorln(err.Error())
		return err
	}
	contents, _ := fileContents(infrastructureBlueprint)
	l.println(contents)
	l.infoln("End " + infrastructureBlueprint)

	_, _, err = runBlueprintUntilConnected(infrastructureBlueprint)
	if err != nil {
		l.infoln("Failed to setup infrastructure")
		l.errorln(err.Error())
		return err
	}

	l.infoln("Booted Quilt")
	l.infoln("Machines")
	machines, _ := queryMachines()
	l.println(fmt.Sprintf("%v", machines))

	return nil
}

func (t tester) runTestSuites() error {
	var err error
	for _, suite := range t.testSuites {
		if e := suite.run(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

type testSuite struct {
	name      string
	blueprint string
	test      string

	output      string
	passed      bool
	timeElapsed time.Duration
}

func (ts *testSuite) run() error {
	testStart := time.Now()
	l := log.testerLogger

	defer func() {
		ts.timeElapsed = time.Since(testStart)
	}()
	defer func() {
		logsPath := filepath.Join(os.Getenv("WORKSPACE"), ts.name+"_debug_logs")
		cmd := exec.Command("quilt", "-v", "debug-logs", "-tar=false",
			"-o="+logsPath, "-all")
		stdout, stderr, err := execCmd(cmd, "DEBUG LOGS")
		if err != nil {
			l.errorln(fmt.Sprintf("Debug logs encountered an error:"+
				" %v\nstdout: %s\nstderr: %s", err, stdout, stderr))
		}
	}()

	l.infoln(fmt.Sprintf("Test Suite: %s", ts.name))
	l.infoln("Start " + ts.name + ".js")
	contents, _ := fileContents(ts.blueprint)
	l.println(contents)
	l.infoln("End " + ts.name + ".js")
	defer l.infoln(fmt.Sprintf("Finished Test Suite: %s", ts.name))

	runBlueprint(ts.blueprint)

	l.infoln("Waiting for containers to start up")
	if err := waitForContainers(ts.blueprint); err != nil {
		l.println(".. Containers never started: " + err.Error())
		ts.passed = false
		return err
	}

	// Wait a little bit longer for any container bootstrapping after boot.
	time.Sleep(90 * time.Second)

	var err error
	if ts.test != "" {
		l.infoln("Starting Test")
		l.println(".. " + filepath.Base(ts.test))

		ts.output, err = runTest(ts.test)
		if err == nil {
			l.println(".... Passed")
			ts.passed = true
		} else {
			l.println(".... Failed")
			ts.passed = false
		}
	}

	return err
}

func waitForContainers(blueprintPath string) error {
	stc, err := blueprint.FromFile(blueprintPath)
	if err != nil {
		return err
	}

	c, err := testerUtil.GetDefaultDaemonClient()
	if err != nil {
		return err
	}
	defer c.Close()

	return util.BackoffWaitFor(func() bool {
		curr, err := c.QueryContainers()
		if err != nil {
			return false
		}

		// Only match containers that have the same blueprint ID, and have been
		// booted.
		key := func(tgtIntf, actualIntf interface{}) int {
			tgt := tgtIntf.(blueprint.Container)
			actual := actualIntf.(db.Container)
			if tgt.ID == actual.BlueprintID && !actual.Created.IsZero() {
				return 0
			}
			return -1
		}
		_, unbooted, _ := join.Join(stc.Containers, curr, key)
		return len(unbooted) == 0
	}, 15*time.Second, 10*time.Minute)
}

func runTest(testPath string) (string, error) {
	output, err := exec.Command(testPath).CombinedOutput()
	if err != nil {
		_, testName := filepath.Split(testPath)
		err = fmt.Errorf("test failed: %s", testName)
	}
	return string(output), err
}

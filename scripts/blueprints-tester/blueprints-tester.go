package main

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/kelda/kelda/scripts/blueprints-tester/tests"
)

func main() {
	testsByName := map[string]func() error{
		"TestReadme":            tests.TestReadme,
		"TestCIBlueprints":      tests.TestCIBlueprints,
		"TestExampleBlueprints": tests.TestExampleBlueprints,
	}

	failed := false
	for name, test := range testsByName {
		// Reset the working directory. Normally this is handled by
		// the go testing framework, but we run these tests manually,
		// because they require an internet connection, and we don't
		// want the build to require an internet connection.
		_, filename, _, _ := runtime.Caller(0)
		workingDir := path.Dir(filename)
		os.Chdir(workingDir)

		if result := test(); result != nil {
			fmt.Printf("FAILED\t%s: %s\n", name, result.Error())
			failed = true
		} else {
			fmt.Printf("PASSED\t%s\n", name)
		}
	}

	if failed {
		os.Exit(1)
	}
	os.Exit(0)
}

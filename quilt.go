//go:generate protoc ./minion/pb/pb.proto --go_out=plugins=grpc:.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	l_mod "log"
	"os"
	"strings"

	"github.com/quilt/quilt/quiltctl"
	"github.com/quilt/quilt/util"

	"google.golang.org/grpc/grpclog"

	log "github.com/Sirupsen/logrus"
)

var quiltCommands = `quilt [-log-level=<level> | -l=<level>] [-H=<listen_address>]
[-verbose | -v] [-log-file=<log_output_file>] [daemon | inspect <blueprint> |
run <blueprint> | minion | stop [<namespace> | -containers] | ps | counters |
ssh <id> [command] | logs <id> | debug-logs <id...> | version]`

var quiltExplanation = `When provided a blueprint, quilt takes responsibility for
deploying it as specified. Alternatively, quilt may be instructed to stop all
deployments or containers in a given namespace, or in the default namespace if
none is provided.`

func main() {
	flag.Usage = func() {
		util.PrintUsageString(quiltCommands, quiltExplanation, nil)
	}
	var logLevelInfo = "logging level (debug, info, warn, error, fatal, or panic)"
	var debugInfo = "turn on debug logging"

	var logOut = flag.String("log-file", "", "log output file (will be overwritten)")
	var logLevel = flag.String("log-level", "info", logLevelInfo)
	var debugOn = flag.Bool("verbose", false, debugInfo)
	flag.StringVar(logLevel, "l", "info", logLevelInfo)
	flag.BoolVar(debugOn, "v", false, debugInfo)
	flag.Parse()

	level, err := parseLogLevel(*logLevel, *debugOn)
	if err != nil {
		fmt.Println(err)
		usage()
	}
	log.SetLevel(level)
	log.SetFormatter(util.Formatter{})

	if *logOut != "" {
		file, err := os.Create(*logOut)
		if err != nil {
			fmt.Printf("Failed to create file %s\n", *logOut)
			os.Exit(1)
		}
		defer file.Close()
		log.SetOutput(file)
	}

	// GRPC spews a lot of useless log messages so we discard its logs, unless
	// we are in debug mode
	grpclog.SetLogger(l_mod.New(ioutil.Discard, "", 0))
	if level == log.DebugLevel {
		grpclog.SetLogger(log.StandardLogger())
	}

	if len(flag.Args()) == 0 {
		usage()
	}

	subcommand := flag.Arg(0)
	if quiltctl.HasSubcommand(subcommand) {
		quiltctl.Run(subcommand, flag.Args()[1:])
	} else {
		usage()
	}
}

func usage() {
	flag.Usage()
	os.Exit(1)
}

// parseLogLevel returns the log.Level type corresponding to the given string
// (case insensitive).
// If no such matching string is found, it returns log.InfoLevel (default) and an error.
func parseLogLevel(logLevel string, debug bool) (log.Level, error) {
	if debug {
		return log.DebugLevel, nil
	}

	logLevel = strings.ToLower(logLevel)
	switch logLevel {
	case "debug":
		return log.DebugLevel, nil
	case "info":
		return log.InfoLevel, nil
	case "warn":
		return log.WarnLevel, nil
	case "error":
		return log.ErrorLevel, nil
	case "fatal":
		return log.FatalLevel, nil
	case "panic":
		return log.PanicLevel, nil
	}
	return log.InfoLevel, fmt.Errorf("bad log level: '%v'", logLevel)
}

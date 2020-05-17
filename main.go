package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/cmd"
	"gotest.tools/gotestsum/cmd/tool"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/testjson"
)

var version = "master"

func main() {
	err := route(os.Args)
	switch err.(type) {
	case nil:
		return
	case *exec.ExitError:
		// go test should already report the error to stderr, exit with
		// the same status code
		os.Exit(ExitCodeWithDefault(err))
	default:
		log.Error(err.Error())
		os.Exit(3)
	}
}

func route(args []string) error {
	name := args[0]
	next, rest := cmd.Next(args[1:])
	switch next {
	case "tool":
		return tool.Run(name+" "+next, rest)
	default:
		return runMain(name, args[1:])
	}
}

func runMain(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		flags.Usage()
		return err
	}
	opts.args = flags.Args()
	setupLogging(opts)

	if opts.version {
		fmt.Fprintf(os.Stdout, "gotestsum version %s\n", version)
		return nil
	}
	return run(opts)
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{
		noSummary:                    newNoSummaryValue(),
		junitTestCaseClassnameFormat: &junitFieldFormatValue{},
		junitTestSuiteNameFormat:     &junitFieldFormatValue{},
		postRunHookCmd:               &commandValue{},
		stdout:                       os.Stdout,
		stderr:                       os.Stderr,
	}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
    %s [flags] [--] [go test flags]

Flags:
`, name)
		flags.PrintDefaults()
		fmt.Fprint(os.Stderr, `
Formats:
    dots                    print a character for each test
    dots-v2                 experimental dots format, one package per line
    pkgname                 print a line for each package
    pkgname-and-test-fails  print a line for each package and failed test output
    testname                print a line for each test and package
    standard-quiet          standard go test format
    standard-verbose        standard go test -v format
`)
	}
	flags.StringVarP(&opts.format, "format", "f",
		lookEnvWithDefault("GOTESTSUM_FORMAT", "short"),
		"print format of test input")
	flags.BoolVar(&opts.rawCommand, "raw-command", false,
		"don't prepend 'go test -json' to the 'go test' command")
	flags.StringVar(&opts.jsonFile, "jsonfile",
		lookEnvWithDefault("GOTESTSUM_JSONFILE", ""),
		"write all TestEvents to file")
	flags.BoolVar(&opts.noColor, "no-color", color.NoColor, "disable color output")
	flags.Var(opts.noSummary, "no-summary",
		"do not print summary of: "+testjson.SummarizeAll.String())
	flags.Var(opts.postRunHookCmd, "post-run-command",
		"command to run after the tests have completed")

	flags.StringVar(&opts.junitFile, "junitfile",
		lookEnvWithDefault("GOTESTSUM_JUNITFILE", ""),
		"write a JUnit XML file")
	flags.Var(opts.junitTestSuiteNameFormat, "junitfile-testsuite-name",
		"format the testsuite name field as: "+junitFieldFormatValues)
	flags.Var(opts.junitTestCaseClassnameFormat, "junitfile-testcase-classname",
		"format the testcase classname field as: "+junitFieldFormatValues)

	flags.IntVar(&opts.rerunFailsMaxAttempts, "rerun-fails-max-attempts", 0,
		"rerun failed tests until each one passes once, or attempts exceeds max")
	flags.IntVar(&opts.rerunFailsMaxInitialFailures, "rerun-fails-max-failures", 10,
		"avoid re-run if initial run had more than this number of failures")

	flags.BoolVar(&opts.debug, "debug", false, "enabled debug logging")
	flags.BoolVar(&opts.version, "version", false, "show version and exit")
	return flags, opts
}

func lookEnvWithDefault(key, defValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defValue
}

type options struct {
	args                         []string
	format                       string
	debug                        bool
	rawCommand                   bool
	jsonFile                     string
	junitFile                    string
	postRunHookCmd               *commandValue
	noColor                      bool
	noSummary                    *noSummaryValue
	junitTestSuiteNameFormat     *junitFieldFormatValue
	junitTestCaseClassnameFormat *junitFieldFormatValue
	rerunFailsMaxAttempts        int
	rerunFailsMaxInitialFailures int
	version                      bool

	// shims for testing
	stdout io.Writer
	stderr io.Writer
}

func setupLogging(opts *options) {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	color.NoColor = opts.noColor
}

func run(opts *options) error {
	ctx := context.Background()
	// TODO: validate opts.args against rerunFailsMaxAttempts

	goTestProc, err := startGoTest(ctx, goTestCmdArgs(opts, rerunOpts{}))
	if err != nil {
		return errors.Wrapf(err, "failed to run %s", strings.Join(goTestProc.cmd.Args, " "))
	}
	defer goTestProc.cancel()

	handler, err := newEventHandler(opts)
	if err != nil {
		return err
	}
	defer handler.Close() // nolint: errcheck
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout:  goTestProc.stdout,
		Stderr:  goTestProc.stderr,
		Handler: handler,
	})
	if err != nil {
		return err
	}
	goTestExitErr := goTestProc.cmd.Wait()
	if opts.rerunFailsMaxAttempts > 0 {
		goTestExitErr = rerunFailed(ctx, opts, exec)
	}

	testjson.PrintSummary(opts.stdout, exec, opts.noSummary.value)
	if err := writeJUnitFile(opts, exec); err != nil {
		return err
	}
	if err := postRunHook(opts, exec); err != nil {
		return err
	}
	return goTestExitErr
}

func runGoTest(ctx context.Context, cmdArgs []string, opts *options) (*testjson.Execution, error) {
	goTestProc, err := startGoTest(ctx, cmdArgs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run %s", strings.Join(goTestProc.cmd.Args, " "))
	}
	defer goTestProc.cancel()

	handler, err := newEventHandler(opts)
	if err != nil {
		return nil, err
	}
	defer handler.Close() // nolint: errcheck
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout:  goTestProc.stdout,
		Stderr:  goTestProc.stderr,
		Handler: handler,
	})
	if err != nil {
		return nil, err
	}
	// Always return a non-nil Execution in this case, it may be used to rerun fails.
	return exec, goTestProc.cmd.Wait()
}

type rerunOpts struct {
	runFlag string
	pkg     string
}

func (o rerunOpts) packageArg(defaultPkg string) string {
	if o.pkg != "" {
		return o.pkg
	}
	return lookEnvWithDefault("TEST_DIRECTORY", defaultPkg)
}

func goTestCmdArgs(opts *options, rerunOpts rerunOpts) []string {
	if opts.rawCommand {
		var result []string
		result = append(result, opts.args...)
		if rerunOpts.runFlag != "" {
			result = append(result, rerunOpts.runFlag)
		}
		if rerunOpts.pkg != "" {
			result = append(result, rerunOpts.pkg)
		}
		return result
	}

	args := opts.args
	result := []string{"go", "test"}

	if len(args) == 0 {
		result = append(result, "-json")
		if rerunOpts.runFlag != "" {
			result = append(result, rerunOpts.runFlag)
		}
		return append(result, rerunOpts.packageArg("./..."))
	}

	if !hasJSONArg(args) {
		result = append(result, "-json")
	}
	if rerunOpts.runFlag != "" {
		// TODO: remove -run arg (add test case)
		result = append(result, rerunOpts.runFlag)
	}
	if rerunOpts.pkg != "" {
		// TODO: broken
	}
	if testPath := rerunOpts.packageArg(""); testPath != "" {
		args = append(args, testPath)
	}
	return append(result, args...)
}

func hasJSONArg(args []string) bool {
	for _, arg := range args {
		if arg == "-json" || arg == "--json" {
			return true
		}
	}
	return false
}

type proc struct {
	cmd    *exec.Cmd
	stdout io.Reader
	stderr io.Reader
	cancel func()
}

func startGoTest(ctx context.Context, args []string) (proc, error) {
	if len(args) == 0 {
		return proc{}, errors.New("missing command to run")
	}

	ctx, cancel := context.WithCancel(ctx)
	p := proc{
		cmd:    exec.CommandContext(ctx, args[0], args[1:]...),
		cancel: cancel,
	}
	log.Debugf("exec: %s", p.cmd.Args)
	var err error
	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return p, err
	}
	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		return p, err
	}
	err = p.cmd.Start()
	if err == nil {
		log.Debugf("go test pid: %d", p.cmd.Process.Pid)
	}
	return p, err
}

func rerunFailed(ctx context.Context, opts *options, exec *testjson.Execution) error {
	failed := len(exec.Failed())
	if failed > opts.rerunFailsMaxInitialFailures {
		return fmt.Errorf(
			"number of test failures (%d) exceeds maximum (%d) set by --rerun-fails-max-failures",
			failed, opts.rerunFailsMaxInitialFailures)
	}

	var lastErr error
	for failed > 0 {
		failed = 0
		for _, pkg := range exec.Packages() {
			rerun := rerunOpts{
				runFlag: goTestRunFlagFromTestCases(exec.Package(pkg).Failed),
				pkg:     pkg,
			}
			cmdArgs := goTestCmdArgs(opts, rerun)
			exec, lastErr = runGoTest(ctx, cmdArgs, opts)
			failed += len(exec.Failed())
		}
	}
	return lastErr
}

func goTestRunFlagFromTestCases(tcs []testjson.TestCase) string {
	buf := new(strings.Builder)
	buf.WriteString("-run=")
	for i, tc := range tcs {
		if i != 0 {
			buf.WriteString("|")
		}
		buf.WriteString(tc.Test)
	}
	return buf.String()
}

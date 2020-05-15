package rerunfailed

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/testjson"
)

func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		flags.Usage()
		return err
	}
	return run(opts)
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
    %s [flags]

Read a json file and rerun failed tests a number of times until they all pass
at least once. The json file can be created with 'gotestsum --jsonfile' or
'go test -json'.

Flags:
`, name)
		flags.PrintDefaults()
	}
	// TODO: GOTESTSUM_JSONFILE env var as default
	flags.StringVar(&opts.jsonfile, "jsonfile", "",
		"path to test2json output, defaults to stdin")
	flags.IntVar(&opts.count, "count", 3, "maximum number of times to rerun a test")
	flags.BoolVar(&opts.debug, "debug", false,
		"enable debug logging.")
	flags.IntVar(&opts.rerunFailsMaxAttempts, "rerun-fails-max-attempts", 2,
		"the number of reruns of failures to perform after a test fails")
	flags.IntVar(&opts.rerunFailsMaxFailures, "rerun-fails-max-failures", 20,
		"do not rerun failures if there are more than this number")
	return flags, opts
}

type options struct {
	jsonfile              string
	count                 int
	rerunFailsMaxFailures int
	rerunFailsMaxAttempts int
	debug                 bool
}

func run(opts *options) error {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	in, err := jsonfileReader(opts.jsonfile)
	if err != nil {
		return fmt.Errorf("failed to read jsonfile: %v", err)
	}
	defer func() {
		if err := in.Close(); err != nil {
			log.Errorf("Failed to close file %v: %v", opts.jsonfile, err)
		}
	}()

	var next = in
	for i := 0; i < opts.count; i++ {
		var err error
		next, err = rerunFailed(next)
		switch {
		case err != nil:
			return fmt.Errorf("failed to run tests: %v", err)
		case next == nil:
			return nil
		}
		defer next.Close()
	}
	// TODO: better error
	return fmt.Errorf("some tests still failing")
}

func rerunFailed(in io.Reader) (io.ReadCloser, error) {
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: in,
		Stderr: bytes.NewReader(nil),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan testjson: %v", err)
	}

	tcs := failedTestCases(exec)
	if len(tcs) == 0 {
		return nil, nil
	}

	return nil
}

func failedTestCases(exec *testjson.Execution) []testjson.TestCase {
	pkgs := exec.Packages()
	tests := make([]testjson.TestCase, 0, len(pkgs))
	for _, pkg := range pkgs {
		tests = append(tests, exec.Package(pkg).Failed...)
	}
	return tests
}

func jsonfileReader(v string) (io.ReadCloser, error) {
	switch v {
	case "", "-":
		return ioutil.NopCloser(os.Stdin), nil
	default:
		return os.Open(v)
	}
}

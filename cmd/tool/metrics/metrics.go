package metrics

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	protocol "github.com/influxdata/line-protocol"
	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/testjson"
)

// Run the metrics command.
func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		usage(os.Stderr, name, flags)
		return err
	}
	return run(opts)
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}
	flags.StringVar(&opts.jsonfile, "jsonfile", os.Getenv("GOTESTSUM_JSONFILE"),
		"path to test2json output, defaults to stdin")
	flags.StringVar(&opts.output, "output", "",
		"path to output file, defaults to stdout")
	flags.Var(&opts.tagSource, "tag-source",
		"set the source for tag values")
	return flags, opts
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags]

Read a json file and write out metrics about the run.

Flags:
`, name)
	flags.SetOutput(out)
	flags.PrintDefaults()
}

type options struct {
	jsonfile  string
	output    string
	tagSource tagSourceValue
}

func run(opts *options) error {
	in, err := jsonfileReader(opts.jsonfile)
	if err != nil {
		return fmt.Errorf("failed to read jsonfile: %v", err)
	}
	defer func() {
		if err := in.Close(); err != nil {
			log.Errorf("Failed to close file %v: %v", opts.jsonfile, err)
		}
	}()

	out, err := outputFileWriter(opts.output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Errorf("Failed to close file %v: %v", opts.output, err)
		}
	}()

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{Stdout: in})
	if err != nil {
		return fmt.Errorf("failed to scan testjson: %v", err)
	}

	// TODO: support other output formats
	return writeMetrics(out, exec, newTagSource(opts.tagSource.String()))
}

func writeMetrics(out io.Writer, exec *testjson.Execution, source tagSource) error {
	e := protocol.NewEncoder(out)
	e.FailOnFieldErr(true)
	e.SetFieldSortOrder(protocol.SortFields)
	for _, tc := range exec.Failed() {
		if _, err := e.Encode(newMetric(tc, source)); err != nil {
			return err
		}
	}
	// TODO: emit metrics for non-failed tests as well?
	return nil
}

func newMetric(tc testjson.TestCase, ts tagSource) protocol.Metric {
	tags := ts.AsMap()
	tags["test.name"] = tc.Package + "." + tc.Test

	fields := map[string]interface{}{
		"elapsed": tc.Elapsed.Nanoseconds(),
		"count":   1, // TODO: is this necessary to sum, or is this provided automatically?
		"result":  "failed",
	}
	metric, err := protocol.New("testcase", tags, fields, tc.Time)
	if err != nil {
		// protocol.New currently never returns an error. Handle the error with
		// a log.Warn in case that changes in the future.
		log.Warnf("unexpected error while creating metric: %v", err)
	}
	return metric
}

func jsonfileReader(v string) (io.ReadCloser, error) {
	switch v {
	case "", "-":
		return ioutil.NopCloser(os.Stdin), nil
	default:
		return os.Open(v)
	}
}

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func outputFileWriter(v string) (io.WriteCloser, error) {
	switch v {
	case "", "-":
		return writeCloser{Writer: os.Stdout}, nil
	default:
		return os.Create(v)
	}
}

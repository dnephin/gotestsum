package metrics

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

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
		if err := in.Close(); err != nil {
			log.Errorf("Failed to close file %v: %v", opts.output, err)
		}
	}()

	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{Stdout: in})
	if err != nil {
		return fmt.Errorf("failed to scan testjson: %v", err)
	}

	return writeMetrics(out, exec, newTagSource(opts.tagSource.String()))
}

func writeMetrics(out io.Writer, exec *testjson.Execution, source tagSource) error {
	for _, _ = range exec.Failed() {

	}
	// TODO: emit metrics for non-failed tests as well?
	return nil
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

package ciaction

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/reaction"
)

// Run the command
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

// TODO: rename if this does not end up using flags.
func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}
	flags.BoolVar(&opts.debug, "debug", false, "enable debug logging.")

	opts.circleCI.token = os.Getenv("CIRCLECI_TOKEN")
	opts.circleCI.workflowID = getWorkflowID()
	opts.circleCI.projectSlug = os.Getenv("CIRCLECI_PROJECT_SLUG")
	opts.circleCI.jobPattern = getEnvWithDefault("CIRCLECI_JOB_PATTERN", "*")
	opts.circleCI.rerunFailsReportPattern = getEnvWithDefault("RERUN_FAILS_PATTERN", "tmp/rerun-fails-report")

	return flags, opts
}

func getWorkflowID() string {
	v := os.Getenv("CIRCLECI_WORKFLOW")
	if !strings.Contains(v, `"workflow-id"`) {
		return v
	}

	type externalID struct {
		Value string `json:"workflow-id"`
	}
	target := &externalID{}
	if err := json.Unmarshal([]byte(v), target); err != nil {
		log.Warnf("failed to parse workflow-id from %v", v)
	}
	return target.Value
}

func getEnvWithDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags]

Fetch artifacts from a CI job, and perform for any actions that match a known
pattern.

Flags:
`, name)
	flags.SetOutput(out)
	flags.PrintDefaults()
}

type options struct {
	circleCI circleCI
	debug    bool
}

type circleCI struct {
	workflowID              string
	jobNum                  int
	token                   string
	projectSlug             string
	jobPattern              string
	rerunFailsReportPattern string
}

func (o options) Validate() error {
	if o.circleCI.jobNum == 0 && o.circleCI.workflowID == "" {
		return fmt.Errorf("one of CIRCLECI_JOB or CIRCLECI_WORKFLOW is required")
	}
	if o.circleCI.token == "" {
		return fmt.Errorf("a CIRCLECI_TOKEN is required")
	}
	if o.circleCI.projectSlug == "" {
		return fmt.Errorf("a CIRCLECI_PROJECT slug is required")
	}
	return nil
}

func run(opts *options) error {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	if err := opts.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	cfg := newCircleCIConfigFromOptions(opts)
	err := reaction.Act(ctx, cfg)
	return err
}

func newCircleCIConfigFromOptions(opts *options) reaction.CircleCIConfig {
	return reaction.CircleCIConfig{
		ProjectSlug: opts.circleCI.projectSlug,
		Token:       opts.circleCI.token,
		Client:      &http.Client{},
		JobNum:      opts.circleCI.jobNum,
		WorkflowID:  opts.circleCI.workflowID,
		JobPattern:  opts.circleCI.jobPattern,
		Actions: reaction.Actions{
			RerunFailsReportPattern: opts.circleCI.rerunFailsReportPattern,
		},
	}
}

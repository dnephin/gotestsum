package testmetrics

import (
	"context"
	"fmt"
	"io"
	"time"

	"gotest.tools/gotestsum/internal/aggregate"
	"gotest.tools/gotestsum/testjson"
)

type Config struct {
	Source   []io.ReadCloser
	Emitter  MetricsEmitter
	Settings MetricConfig
	Logger   Logger
}

type MetricsEmitter interface {
	Emit(ctx context.Context, metrics Metrics) error
}

type Logger interface {
	Info(...interface{})
}

func Produce(ctx context.Context, cfg Config) error {
	exec, err := buildExecution(cfg.Source)
	if err != nil || exec == nil {
		return err
	}

	metrics, err := metricsFromExec(cfg.Settings, exec)
	if err != nil {
		return err
	}

	// TODO: set Metrics.Tags based on data from CircleCI job

	cfg.Logger.Info("Writing metrics to influxDB")

	return cfg.Emitter.Emit(ctx, metrics)
}

type Metrics struct {
	Slowest []testjson.TestCase
	Failed  []testjson.TestCase
	Tags    map[string]string
}

type MetricConfig struct {
	// MaxFailuresThreshold. If the test run has more than this number of failures
	// no metrics will be emitted. This threshold is used to avoid sending
	// metrics for test runs that failed due to real bugs or infrastructure problems.
	MaxFailuresThreshold int

	// SlowTestThreshold is used to exclude fast tests from being remoted in the
	// list of slowest tests. Any test which runs in less time than this threshold
	// will not be added to Metrics.Slowest.
	SlowTestThreshold time.Duration

	// MaxSlowTests is the maximum number of slow tests to return in Metrics.Slowest.
	MaxSlowTests int

	// TODO: patterns for BranchCategory
}

func metricsFromExec(cfg MetricConfig, exec *testjson.Execution) (Metrics, error) {
	var m Metrics

	failed := exec.Failed()
	if len(failed) > cfg.MaxFailuresThreshold {
		return m, fmt.Errorf("failures (%d) exceeded threshold (%d)", len(failed), cfg.MaxFailuresThreshold)
	}
	m.Failed = failed

	m.Slowest = aggregate.Slowest(exec, cfg.SlowTestThreshold)
	if len(m.Slowest) > cfg.MaxSlowTests {
		m.Slowest = m.Slowest[:cfg.MaxSlowTests]
	}

	return m, nil
}

// buildExecution will close all input io.ReadClosers before returning.
func buildExecution(in []io.ReadCloser) (*testjson.Execution, error) {
	var lastErr error
	scanCfg := testjson.ScanConfig{}
	for _, reader := range in {
		scanCfg.Stdout = reader

		exec, err := testjson.ScanTestOutput(scanCfg)
		if err != nil {
			lastErr = err
		}
		scanCfg.Execution = exec
		reader.Close() // nolint: errcheck
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to scan test output: %w", lastErr)
	}
	return scanCfg.Execution, nil
}

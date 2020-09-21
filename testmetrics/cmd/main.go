package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gotest.tools/gotestsum/testmetrics"
)

func main() {
	if err := run(os.Args[0]); err != nil {
		log.Print("ERROR: ", err.Error())
		os.Exit(1)
	}
}

func run(_ string) error {
	ctx := context.Background()
	log.SetFlags(0)

	env := &env{}
	client := &http.Client{}

	cfg := testmetrics.Config{
		Job: testmetrics.CircleCIJob{
			ProjectSlug:  env.LookupEnv("CIRCLECI_PROJECT_SLUG"),
			Job:          env.LookupInt("CIRCLECI_JOB"),
			Token:        env.LookupEnv("CIRCLECI_API_TOKEN"),
			ArtifactGlob: env.LookupEnv("CIRCLECI_ARTIFACT_GLOB"),
			Client:       client,
		},
		Emitter: testmetrics.InfluxDBEmitter{
			Addr:   env.LookupEnv("INFLUX_HOST"),
			Bucket: env.LookupEnv("INFLUX_BUCKET_ID"),
			Org:    env.LookupEnv("INFLUX_ORG_ID"),
			Token:  env.LookupEnv("INFLUX_TOKEN"),
			Client: client,
		},
		Settings: testmetrics.MetricConfig{
			MaxFailuresThreshold: 10,
			SlowTestThreshold:    time.Second,
			MaxSlowTests:         10,
		},
		Logger: logger{},
	}
	if err := env.Err(); err != nil {
		return err
	}

	return testmetrics.Produce(ctx, cfg)
}

type logger struct{}

func (l logger) Info(i ...interface{}) {
	log.Print(append([]interface{}{"INFO: "}, i...)...)
}

var _ testmetrics.Logger = logger{}

type env struct {
	errs []error
}

func (e *env) LookupEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		e.errs = append(e.errs, fmt.Errorf("missing required value for %v", key))
	}
	return v
}

func (e *env) LookupInt(key string) int {
	i := e.LookupEnv(key)
	if i == "" {
		return 0
	}
	v, err := strconv.Atoi(i)
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("invalid int for %v: %w", key, err))
	}
	return v
}

func (e *env) Err() error {
	return fmtErrors("multiple errors while loading config from env", e.errs)
}

func fmtErrors(msg string, errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		b := new(strings.Builder)

		for _, err := range errs {
			b.WriteString("\n   ")
			b.WriteString(err.Error())
		}
		return fmt.Errorf(msg+":%s\n", b.String())
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gotest.tools/gotestsum/testmetrics"
)

func main() {
	if err := run(os.Args); err != nil {
		log.Print("ERROR: ", err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	ctx := context.Background()
	log.SetFlags(0)

	if len(args) < 2 {
		return fmt.Errorf("job ID is required")
	}

	job, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalud job ID %v: %w", args[1], err)
	}

	cfg := testmetrics.Config{
		Job: testmetrics.CircleCIJob{
			ProjectSlug:  os.Getenv("CIRCLECI_PROJECT_SLUG"),
			Job:          job,
			Token:        os.Getenv("CIRCLECI_API_TOKEN"),
			ArtifactGlob: os.Getenv("CIRCLECI_ARTIFACT_GLOB"),
		},
		Target: testmetrics.InfluxDBTarget{
			Addr:   os.Getenv("INFLUX_HOST"),
			Bucket: os.Getenv("INFLUX_BUCKET_ID"),
			Org:    os.Getenv("INFLUX_ORG_ID"),
			Token:  os.Getenv("INFLUX_TOKEN"),
		},
		Settings: testmetrics.MetricConfig{
			MaxFailuresThreshold: 10,
			SlowTestThreshold:    time.Second,
			MaxSlowTests:         10,
		},
		Logger: logger{},
	}

	return testmetrics.Produce(ctx, cfg)
}

type logger struct{}

func (l logger) Info(i ...interface{}) {
	log.Print(append([]interface{}{"INFO: "}, i...)...)
}

var _ testmetrics.Logger = logger{}

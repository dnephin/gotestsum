package testmetrics

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"gotest.tools/gotestsum/testjson"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/skip"
)

func TestWriteInfluxData(t *testing.T) {
	token := os.Getenv("INFLUX_TOKEN")
	skip.If(t, token == "", "INFLUX_TOKEN env var is required")
	t.Skip("skip to avoid hitting API rate limit")

	target := InfluxDBEmitter{
		Addr:   os.Getenv("INFLUX_HOST"),
		Bucket: os.Getenv("INFLUX_BUCKET_ID"),
		Org:    os.Getenv("INFLUX_ORG_ID"),
		Token:  token,
		Client: &http.Client{},
	}

	body := golden.Get(t, "expected-encode-metrics")
	ctx := context.Background()
	err := writeInfluxData(ctx, target, bytes.NewReader(body))
	assert.NilError(t, err)
}

func TestEncodeMetrics(t *testing.T) {
	date := time.Date(2020, 2, 2, 20, 20, 20, 4159, time.UTC)
	metrics := Metrics{
		Failed: []testjson.TestCase{
			{Package: "pkg1", Test: "TestFair", Elapsed: 3200 * time.Millisecond, Time: date},
			{Package: "pkg2", Test: "TestSoon", Elapsed: 1424 * time.Millisecond, Time: date.Add(time.Minute)},
		},
		Slowest: []testjson.TestCase{
			{Package: "pkg1", Test: "TestFair", Elapsed: 3200 * time.Millisecond, Time: date},
			{Package: "pkg3", Test: "TestOk", Elapsed: 3512 * time.Millisecond, Time: date.Add(5 * time.Second)},
		},
		Tags: map[string]string{
			"branch":          "master",
			"branch_category": "trunk",
			"repo":            "example.com/org/project",
		},
	}

	out, err := encodeMetrics(metrics)
	assert.NilError(t, err)
	golden.Assert(t, out.String(), "expected-encode-metrics")
}

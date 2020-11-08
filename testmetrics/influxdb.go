package testmetrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	protocol "github.com/influxdata/line-protocol"
	"gotest.tools/gotestsum/log"
	"gotest.tools/gotestsum/testjson"
)

type InfluxDBEmitter struct {
	Addr   string
	Bucket string
	Org    string
	Token  string
	Client httpDoer
}

func (e InfluxDBEmitter) Emit(ctx context.Context, metrics Metrics) error {
	encoded, err := encodeMetrics(metrics)
	if err != nil {
		return err
	}

	return writeInfluxData(ctx, e, encoded)
}

func writeInfluxData(ctx context.Context, target InfluxDBEmitter, data io.Reader) error {
	v := url.Values{}
	v.Add("bucket", target.Bucket)
	v.Add("org", target.Org)
	v.Add("precision", "ns")
	u := target.Addr + "/api/v2/write?" + v.Encode()

	req, err := http.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "text/plain; charset=utf-8")
	req.Header.Add("Authorization", "Token "+target.Token)

	resp, err := target.Client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		msg := readBodyError(resp.Body)
		return fmt.Errorf("failed to write influx data: %v %v", resp.Status, msg)
	}
	return nil
}

func readBodyError(body io.Reader) string {
	msg, err := ioutil.ReadAll(body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %v", err)
	}
	return string(msg)
}

func encodeMetrics(metric Metrics) (*bytes.Buffer, error) {
	out := new(bytes.Buffer)
	e := protocol.NewEncoder(out)
	e.FailOnFieldErr(true)
	e.SetFieldSortOrder(protocol.SortFields)
	for _, tc := range metric.Failed {
		if _, err := e.Encode(newInfluxMetric(tc, "failed", metric.Tags)); err != nil {
			return nil, err
		}
	}

	for _, tc := range metric.Slowest {
		if _, err := e.Encode(newInfluxMetric(tc, "slow", metric.Tags)); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func newInfluxMetric(tc testjson.TestCase, result string, tags map[string]string) protocol.Metric {
	fields := map[string]interface{}{
		"elapsed": tc.Elapsed.Nanoseconds(),
		// TODO: is this necessary to sum, or is this provided automatically?
		// TODO: should this be a tag?
		"count": 1,
	}
	metric, err := protocol.New("testcase", tags, fields, tc.Time)
	if err != nil {
		// protocol.New currently never returns an error. Handle the error with
		// a log.Warn in case that changes in the future.
		log.Warnf("unexpected error while creating metric: %v", err)
	}

	// Add the tags to the constructed metric to avoid creating another copy
	// of the tags map. protocol.New makes a copy.
	metric.AddTag("test.name", tc.Package+"."+tc.Test.Name())
	metric.AddTag("result", result)
	return metric
}

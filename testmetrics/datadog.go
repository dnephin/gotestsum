package testmetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type DataDogEmitter struct {
	APIKey string
	Client httpDoer
}

func (d DataDogEmitter) Emit(ctx context.Context, metrics Metrics) error {
	data := newDatadogRequest(metrics)
	return writeDatadogSeries(ctx, d, data)
}

func writeDatadogSeries(ctx context.Context, d DataDogEmitter, data io.Reader) error {
	v := url.Values{}
	v.Add("api_key", d.APIKey)
	u := urlDatadogSeries + "?" + v.Encode()

	req, err := http.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := d.Client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		msg := readBodyError(resp.Body)
		return fmt.Errorf("failed to write datadog series: %v %v", resp.Status, msg)
	}
	return nil
}

var urlDatadogSeries = "https://api.datadoghq.com/api/v1/series"

func newDatadogRequest(metrics Metrics) datadogRequest {
	tags := datadogTags(metrics.Tags)
	series := make([]datadogSeries, 0, len(metrics.Failed)+len(metrics.Slowest))
	for _, tc := range metrics.Failed {
		series = append(series, datadogSeries{
			Host:     datadogHost,
			Interval: 1,
			Metric:   "testcase.failed",
			Tags:     tags,
			Type:     "count",
			Points:   []datadogPoint{{Timestamp: tc.Time, Value: 1}},
		})
	}
	for _, tc := range metrics.Slowest {
		series = append(series, datadogSeries{
			Host:     datadogHost,
			Interval: 1,
			Metric:   "testcase.slow",
			Tags:     tags,
			Type:     "histogram",
			Points:   []datadogPoint{{Timestamp: tc.Time, Value: float64(tc.Elapsed)}},
		})
	}
	return datadogRequest{Series: series}
}

var datadogHost = "ci"

func datadogTags(t map[string]string) []string {
	result := make([]string, 0, len(t))
	for k, v := range t {
		result = append(result, fmt.Sprintf("%v:%v", k, v))
	}
	return result
}

type datadogRequest struct {
	Series []datadogSeries `json:"series"`
}

// Read provides the datadogRequest as json encoded bytes.
func (d datadogRequest) Read(p []byte) (n int, err error) {
	r, err := json.Marshal(d)
	if err != nil {
		return 0, err
	}
	return bytes.NewReader(r).Read(p)
}

var _ io.Reader = (*datadogRequest)(nil)

type datadogSeries struct {
	Host     string         `json:"host"`
	Interval int64          `json:"interval"`
	Metric   string         `json:"metric"`
	Tags     []string       `json:"tags"`
	Type     string         `json:"type"`
	Points   []datadogPoint `json:"points"`
}

type datadogPoint struct {
	Timestamp time.Time
	Value     float64
}

func (p datadogPoint) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]float64{float64(p.Timestamp.UnixNano()), p.Value})
}

package testmetrics

import (
	"context"
	"time"
)

type DataDogEmitter struct {
	APIKey string
	Client httpDoer
}

func (d DataDogEmitter) Emit(ctx context.Context, metrics Metrics) error {

}

var urlDatadogSeries = "https://api.datadoghq.com/api/v1/series"

type datadogRequest struct {
	Series []datadogSeries `json:"series"`
}

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

// TODO: custom JSON marshal for datadogPoint

package testmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

type CircleCIJob struct {
	ProjectSlug  string
	Job          int
	Token        string
	ArtifactGlob string
}

// getCircleCIJsonFiles for a single CircleCI job. If the returned error is nil the
// ReadClosers must be closed by the caller.
func getCircleCIJsonFiles(ctx context.Context, client httpDoer, job CircleCIJob) ([]io.ReadCloser, error) {
	arts, err := getArtifactURLs(ctx, client, job)
	if err != nil {
		return nil, err
	}

	urls, err := filterArtifactURLs(*arts, job.ArtifactGlob)
	if err != nil {
		return nil, err
	}

	result := make([]io.ReadCloser, 0, len(urls))
	for _, u := range urls {
		body, err := getArtifact(ctx, client, u)
		if err != nil {
			return nil, err
		}
		result = append(result, body)
	}
	return result, nil
}

type responseArtifact struct {
	Items []responseArtifactItem `json:"items"`
}

type responseArtifactItem struct {
	Path string `json:"path"`
	URL  string `json:"url"`
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

const circleArtifactsURL = `https://circleci.com/api/v2/project/%s/%d/artifacts`

func getArtifactURLs(ctx context.Context, c httpDoer, job CircleCIJob) (*responseArtifact, error) {
	u := fmt.Sprintf(circleArtifactsURL, url.PathEscape(job.ProjectSlug), job.Job)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Circle-Token", job.Token)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// TODO: check status code

	arts := &responseArtifact{}
	err = json.NewDecoder(resp.Body).Decode(arts)
	return arts, err
}

func filterArtifactURLs(arts responseArtifact, glob string) ([]string, error) {
	var result []string
	for _, item := range arts.Items {
		switch matched, err := path.Match(glob, item.Path); {
		case err != nil:
			return nil, err
		case !matched:
			continue
		}
		result = append(result, item.URL)
	}
	return result, nil
}

func getArtifact(ctx context.Context, c httpDoer, url string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

package reaction

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
)

type CircleCIJob struct {
	ProjectSlug  string
	Job          int
	Token        string
	ArtifactGlob string
	Client       httpDoer
}

// getCircleCIJsonFiles for a single CircleCI job. If the returned error is nil the
// ReadClosers must be closed by the caller.
func getCircleCIJsonFiles(ctx context.Context, job CircleCIJob) ([]io.ReadCloser, error) {
	arts, err := getArtifactURLs(ctx, job.Client, job)
	if err != nil {
		return nil, err
	}

	urls, err := filterArtifactURLs(*arts, job.ArtifactGlob)
	if err != nil {
		return nil, err
	}

	result := make([]io.ReadCloser, 0, len(urls))
	for _, u := range urls {
		body, err := getArtifact(ctx, job.Client, u)
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

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		msg := readBodyError(resp.Body)
		return nil, fmt.Errorf("failed to query artifact URLs: %v %v", resp.Status, msg)
	}

	arts := &responseArtifact{}
	err = json.NewDecoder(resp.Body).Decode(arts)
	return arts, err
}

func readBodyError(body io.Reader) string {
	msg, err := ioutil.ReadAll(body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %v", err)
	}
	return string(msg)
}

func filterArtifactURLs(arts responseArtifact, glob string) ([]string, error) {
	result := make([]string, 0, len(arts.Items))
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

// getArtifact from url. The caller must close the returned ReadCloser.
//
// nolint: bodyclose
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

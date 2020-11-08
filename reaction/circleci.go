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

type CircleCIConfig struct {
	ProjectSlug string
	Token       string
	Client      httpDoer

	ArtifactPattern string
	JobPattern      string

	JobNum     int
	WorkflowID string
}

// getJsonFilesFromJob for a single CircleCI job. If the returned error is nil the
// ReadClosers must be closed by the caller.
func getJsonFilesFromJob(ctx context.Context, cfg CircleCIConfig) ([]io.ReadCloser, error) {
	req := artifactURLRequest{
		ProjectSlug: cfg.ProjectSlug,
		JobNum:      cfg.JobNum,
		Token:       cfg.Token,
	}
	arts, err := getArtifactURLs(ctx, cfg.Client, req)
	if err != nil {
		return nil, err
	}

	urls, err := filterArtifactURLs(*arts, cfg.ArtifactPattern)
	if err != nil {
		return nil, err
	}

	result := make([]io.ReadCloser, 0, len(urls))
	for _, u := range urls {
		body, err := getArtifact(ctx, cfg.Client, u)
		if err != nil {
			return nil, err
		}
		result = append(result, body)
	}
	return result, nil
}

// getJsonFilesFromWorkflow for projects with Github Checks enabled. If the
// returned error is nil the ReadClosers must be closed by the caller.
func getJsonFilesFromWorkflow(ctx context.Context, cfg CircleCIConfig) ([]jobArtifacts, error) {
	jobs, err := getWorkflowJobs(ctx, cfg.Client, workflowJobsRequest{
		WorkflowID: cfg.WorkflowID,
		Token:      cfg.Token,
	})
	if err != nil {
		return nil, err
	}

	var result []jobArtifacts
	for _, job := range jobs {
		switch matched, err := path.Match(cfg.JobPattern, job.Name); {
		case err != nil:
			// TODO: close existing readers
			return nil, err
		case !matched:
			continue
		}

		cfg.JobNum = job.Num
		files, err := getJsonFilesFromJob(ctx, cfg)
		if err != nil {
			// TODO: close existing readers
			return nil, err
		}

		result = append(result, jobArtifacts{Job: job.Name, Files: files})
	}
	return result, nil
}

type jobArtifacts struct {
	Job   string
	Files []io.ReadCloser
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

type artifactURLRequest struct {
	ProjectSlug string
	JobNum      int
	Token       string
}

const circleArtifactsURL = `https://circleci.com/api/v2/project/%s/%d/artifacts`

func getArtifactURLs(ctx context.Context, c httpDoer, opts artifactURLRequest) (*responseArtifact, error) {
	u := fmt.Sprintf(circleArtifactsURL, url.PathEscape(opts.ProjectSlug), opts.JobNum)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Circle-Token", opts.Token)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := statusError(resp); err != nil {
		return nil, fmt.Errorf("failed to query artifact URLs: %w", err)
	}
	arts := &responseArtifact{}
	err = json.NewDecoder(resp.Body).Decode(arts)
	return arts, err
}

func statusError(resp *http.Response) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		msg := readBodyError(resp.Body)
		return fmt.Errorf("http request failed: %v %v", resp.Status, msg)
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

type workflowJobsRequest struct {
	WorkflowID string
	Token      string
}

const circleWorkflowJobsURL = `https://circleci.com/api/v2/workflow/%s/job`

func getWorkflowJobs(ctx context.Context, c httpDoer, opts workflowJobsRequest) ([]workflowJob, error) {
	u := fmt.Sprintf(circleWorkflowJobsURL, url.PathEscape(opts.WorkflowID))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Circle-Token", opts.Token)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := statusError(resp); err != nil {
		return nil, fmt.Errorf("failed to get workflow jobs: %w", err)
	}
	return decodeWorkflowJobs(resp.Body)
}

type workflowJob struct {
	Name string `json:"name"`
	Num  int    `json:"job_number"`
}

func decodeWorkflowJobs(body io.ReadCloser) ([]workflowJob, error) {
	type response struct {
		Items []workflowJob
	}
	var out response
	err := json.NewDecoder(body).Decode(&out)
	return out.Items, err
}

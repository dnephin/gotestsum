package metrics

import (
	"os"

	"gotest.tools/gotestsum/log"
)

func newTagSource(v string) tagSource {
	switch v {
	case "auto":
		switch {
		case os.Getenv("CIRCLECI") != "":
			return newTagSource("circleci")
		default:
			log.Warnf("Failed to auto-detect tag source, defaulting to env")
			return newTagSource("env")
		}
	case "circleci":
		return tagSource{
			GitBranch: os.Getenv("CIRCLE_BRANCH"),
			GitRepo:   os.Getenv("CIRCLE_REPOSITORY_URL"),
			CIJob:     os.Getenv("CIRCLE_JOB"),
		}
	case "env":
		return tagSource{
			GitBranch: os.Getenv("GOTESTSUM_METRIC_TAG_GITBRANCH"),
			GitRepo:   os.Getenv("GOTESTSUM_METRIC_TAG_GITREPO"),
			CIJob:     os.Getenv("GOTESTSUM_METRIC_TAG_CIJOB"),
		}
	}
	panic("programming error: tag source " + v + " not implemented")
}

type tagSource struct {
	GitBranch string
	GitRepo   string
	CIJob     string
	// TODO: allow extra tags?
}

// AsMap return a new map with all the tags. The returned map can be mutated by
// the caller.
func (ts tagSource) AsMap() map[string]string {
	return map[string]string{
		// TODO: normalized branch to pulls+feature/release/master/tags
		"git.branch": ts.GitBranch,
		"git.repo":   ts.GitRepo,
		"ci.job":     ts.CIJob,
	}
}

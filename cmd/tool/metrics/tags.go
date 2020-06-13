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
			return tagSourceCircleCI{}
		default:
			log.Warnf("Failed to auto-detect tag source, defaulting to env")
			return tagSourceEnv{}
		}
	case "circleci":
		return tagSourceCircleCI{}
	case "env":
		return tagSourceEnv{}
	}
	panic("programming error: tag source " + v + " not implemented")
}

type tagSource interface {
	Tags() Tags
}

type Tags struct {
	GitBranch string
	GitRepo   string
	CIJob     string
}

type tagSourceEnv struct{}

func (t tagSourceEnv) Tags() Tags {
	return Tags{
		GitBranch: os.Getenv("GOTESTSUM_METRIC_TAG_GITBRANCH"),
		GitRepo:   os.Getenv("GOTESTSUM_METRIC_TAG_GITREPO"),
		CIJob:     os.Getenv("GOTESTSUM_METRIC_TAG_CIJOB"),
	}
}

type tagSourceCircleCI struct{}

func (t tagSourceCircleCI) Tags() Tags {
	return Tags{
		GitBranch: os.Getenv("CIRCLE_BRANCH"),
		GitRepo:   os.Getenv("CIRCLE_REPOSITORY_URL"),
		CIJob:     os.Getenv("CIRCLE_JOB"),
	}
}

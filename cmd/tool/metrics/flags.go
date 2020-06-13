package metrics

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

type tagSourceValue string

func (t *tagSourceValue) String() string {
	if t == nil || *t == "" {
		return "auto"
	}
	return string(*t)
}

func (t *tagSourceValue) Set(s string) error {
	switch strings.ToLower(s) {
	case "", "auto":
		*t = "auto"
	case "env":
		*t = "env"
	case "circleci":
		*t = "circleci"
	default:
		return fmt.Errorf("unsupported tag source: %v", s)
	}
	return nil
}

func (t *tagSourceValue) Type() string {
	return "string"
}

var _ pflag.Value = (*tagSourceValue)(nil)

package metrics

import (
	"io/ioutil"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"
)

func TestE2E(t *testing.T) {
	type testCase struct {
		name string
		args []string
	}
	tmpDir := fs.NewDir(t, t.Name())
	defer tmpDir.Remove()

	fn := func(t *testing.T, tc testCase) {
		envVars := map[string]string{
			"GOTESTSUM_METRIC_TAG_GITBRANCH": "the-branch",
			"GOTESTSUM_METRIC_TAG_GITREPO":   "example.com/org/the-repo",
			"GOTESTSUM_METRIC_TAG_CIJOB":     "",
		}
		defer env.PatchAll(t, envVars)()

		out := tmpDir.Join(tc.name)
		args := append(tc.args, "--output="+out)

		err := Run("gotestsum tool metrics", args)
		assert.NilError(t, err)

		raw, err := ioutil.ReadFile(out)
		assert.NilError(t, err)

		golden.Assert(t, string(raw), t.Name())
	}
	var testCases = []testCase{
		{
			name: "influxdb line protocol out, tags from env",
			args: []string{
				// TODO: use a jsonfile with more failures, and better data
				"--jsonfile=../../../testjson/testdata/go-test-json.out",
				"--tag-source=env",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("too slow for short run")
			}
			fn(t, tc)
		})
	}
}

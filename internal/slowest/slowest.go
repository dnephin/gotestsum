package slowest

import (
	"sort"
	"time"

	"gotest.tools/gotestsum/testjson"
)

// TestCasesFromExec returns a slice of all tests with an elapsed time greater than
// threshold. The slice is sorted by Elapsed time in descending order (slowest
// test first).
//
// If there are multiple runs of a TestCase, all of them will be represented
// by a single TestCase with the median elapsed time in the returned slice.
func TestCasesFromExec(exec *testjson.Execution, threshold time.Duration) []testjson.TestCase {
	if threshold == 0 {
		return nil
	}
	pkgs := exec.Packages()
	tests := make([]testjson.TestCase, 0, len(pkgs))
	for _, pkg := range pkgs {
		pkgTests := aggregateTestCases(exec.Package(pkg).TestCases())
		tests = append(tests, pkgTests...)
	}
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Elapsed > tests[j].Elapsed
	})
	end := sort.Search(len(tests), func(i int) bool {
		return tests[i].Elapsed < threshold
	})
	return tests[:end]
}

// collectTestCases maps all test cases by name, and if there is more than one
// instance of a TestCase, finds the median elapsed time for all the runs.
//
// All cases are assumed to be part of the same package.
func aggregateTestCases(cases []testjson.TestCase) []testjson.TestCase {
	if len(cases) < 2 {
		return cases
	}
	pkg := cases[0].Package
	// nolint: prealloc // size is not predictable
	m := make(map[string][]time.Duration)
	for _, tc := range cases {
		m[tc.Test] = append(m[tc.Test], tc.Elapsed)
	}
	result := make([]testjson.TestCase, 0, len(m))
	for name, timing := range m {
		result = append(result, testjson.TestCase{
			Package: pkg,
			Test:    name,
			Elapsed: median(timing),
		})
	}
	return result
}

func median(times []time.Duration) time.Duration {
	switch len(times) {
	case 0:
		return 0
	case 1:
		return times[0]
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})
	return times[len(times)/2]
}

package testjson

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
	"gotest.tools/assert/opt"
	"gotest.tools/golden"
	"gotest.tools/skip"
)

//go:generate ./generate.sh

type fakeHandler struct {
	inputName string
	formatter EventFormatter
	out       *bytes.Buffer
	err       *bytes.Buffer
}

func (s *fakeHandler) Config(t *testing.T) ScanConfig {
	return ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, s.inputName+".out")),
		Stderr:  bytes.NewReader(golden.Get(t, s.inputName+".err")),
		Handler: s,
	}
}

func newFakeHandler(handler EventFormatter, inputName string) *fakeHandler {
	return &fakeHandler{
		inputName: inputName,
		formatter: handler,
		out:       new(bytes.Buffer),
		err:       new(bytes.Buffer),
	}
}

func (s *fakeHandler) Event(event TestEvent, execution *Execution) error {
	line, err := s.formatter(event, execution)
	s.out.WriteString(line)
	return err
}

func (s *fakeHandler) Err(text string) error {
	s.err.WriteString(text + "\n")
	return nil
}

func patchPkgPathPrefix(val string) func() {
	var oldVal string
	oldVal, pkgPathPrefix = pkgPathPrefix, val
	return func() { pkgPathPrefix = oldVal }
}

func isGoWithModules() bool {
	version := runtime.Version()
	return strings.HasPrefix(version, "go1.11") && os.Getenv("GO111MODULE") != ""
}

func TestRelativePackagePath(t *testing.T) {
	skip.If(t, isGoWithModules, "no known way of getting package path yet")
	t.Log(runtime.Version())
	relPath := relativePackagePath(
		"gotest.tools/gotestsum/testjson/extra/relpath")
	assert.Equal(t, relPath, "extra/relpath")

	relPath = relativePackagePath(
		"gotest.tools/gotestsum/testjson")
	assert.Equal(t, relPath, ".")
}

func TestGetPkgPathPrefix(t *testing.T) {
	skip.If(t, isGoWithModules, "no known way of getting package path yet")
	assert.Equal(t, pkgPathPrefix, "gotest.tools/gotestsum/testjson")
}

func TestScanTestOutputWithShortVerboseFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandler(shortVerboseFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-verbose-format.out")
	golden.Assert(t, shim.err.String(), "short-verbose-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

var expectedExecution = &Execution{
	started: time.Now(),
	errors:  []string{"internal/broken/broken.go:5:21: undefined: somepackage"},
	packages: map[string]*Package{
		"github.com/gotestyourself/gotestyourself/testjson/internal/good": {
			Total: 18,
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			action: ActionPass,
		},
		"github.com/gotestyourself/gotestyourself/testjson/internal/stub": {
			Total: 28,
			Failed: []TestCase{
				{Test: "TestFailed"},
				{Test: "TestFailedWithStderr"},
				{Test: "TestNestedWithFailure/c"},
				{Test: "TestNestedWithFailure"},
			},
			Skipped: []TestCase{
				{Test: "TestSkipped"},
				{Test: "TestSkippedWitLog"},
			},
			action: ActionFail,
		},
		"github.com/gotestyourself/gotestyourself/testjson/internal/badmain": {
			action: ActionFail,
		},
	},
}

var cmpExecutionShallow = gocmp.Options{
	gocmp.AllowUnexported(Execution{}, Package{}),
	gocmp.FilterPath(stringPath("started"), opt.TimeWithThreshold(10*time.Second)),
	cmpPackageShallow,
}

var cmpPackageShallow = gocmp.Options{
	// TODO: use opt.PathField(Package{}, "output")
	gocmp.FilterPath(stringPath("packages.output"), gocmp.Ignore()),
	gocmp.FilterPath(stringPath("packages.Passed"), gocmp.Ignore()),
	gocmp.Comparer(func(x, y TestCase) bool {
		return x.Test == y.Test
	}),
}

func stringPath(spec string) func(gocmp.Path) bool {
	return func(path gocmp.Path) bool {
		return path.String() == spec
	}
}

func TestScanTestOutputWithDotsFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandler(dotsFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "dots-format.out")
	golden.Assert(t, shim.err.String(), "dots-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithShortFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandler(shortFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "short-format.out")
	golden.Assert(t, shim.err.String(), "short-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithStandardVerboseFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandler(standardVerboseFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "go-test-verbose.out")
	golden.Assert(t, shim.err.String(), "go-test-verbose.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

func TestScanTestOutputWithStandardQuietFormat(t *testing.T) {
	defer patchPkgPathPrefix("github.com/gotestyourself/gotestyourself")()

	shim := newFakeHandler(standardQuietFormat, "go-test-json")
	exec, err := ScanTestOutput(shim.Config(t))

	assert.NilError(t, err)
	golden.Assert(t, shim.out.String(), "standard-quiet-format.out")
	golden.Assert(t, shim.err.String(), "standard-quiet-format.err")
	assert.DeepEqual(t, exec, expectedExecution, cmpExecutionShallow)
}

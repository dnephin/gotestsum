// +build testdata

package flaky

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"testing"

	"gotest.tools/assert"
)

var seed int
var seedfile = "/tmp/gotestsum-flaky-seedfile"
var once = new(sync.Once)

func setup(t *testing.T) {
	once.Do(func() {
		raw, _ := ioutil.ReadFile(seedfile)
		seed = parseSeed(raw)

		err := ioutil.WriteFile(seedfile, []byte(strconv.Itoa(seed+1)), 0644)
		assert.NilError(t, err)
	})
	fmt.Fprintln(os.Stderr, "SEED: ", seed)
}

func parseSeed(r []byte) int {
	n, err := strconv.ParseInt(string(r), 10, 64)
	if err != nil {
		return 0
	}
	return int(n)
}

func TestAlwaysPasses(t *testing.T) {
}

func TestFailsRarely(t *testing.T) {
	setup(t)
	if seed%10 == 3 {
		t.Fatal("not this time")
	}
}

func TestFailsSometimes(t *testing.T) {
	setup(t)
	if seed%5 == 2 {
		t.Fatal("not this time")
	}
}

func TestFailsOften(t *testing.T) {
	setup(t)
	if seed%5 != 1 {
		t.Fatal("not this time")
	}
}

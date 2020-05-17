package main

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
)

func TestGoTestCmdArgs(t *testing.T) {
	type testCase struct {
		opts      *options
		rerunOpts rerunOpts
		env       []string
		expected  []string
	}
	fn := func(t *testing.T, tc testCase) {
		defer env.PatchAll(t, env.ToMap(tc.env))()
		actual := goTestCmdArgs(tc.opts, tc.rerunOpts)
		assert.DeepEqual(t, actual, tc.expected)
	}
	var testcases = map[string]testCase{
		"raw command": {
			opts: &options{
				rawCommand: true,
				args:       []string{"./script", "-test.timeout=20m"},
			},
			expected: []string{"./script", "-test.timeout=20m"},
		},
		"no args": {
			opts:     &options{},
			expected: []string{"go", "test", "-json", "./..."},
		},
		"TEST_DIRECTORY env var no args": {
			opts:     &options{},
			env:      []string{"TEST_DIRECTORY=testdir"},
			expected: []string{"go", "test", "-json", "testdir"},
		},
		"TEST_DIRECTORY env var with args": {
			opts: &options{
				args: []string{"-tags=integration"},
			},
			env:      []string{"TEST_DIRECTORY=testdir"},
			expected: []string{"go", "test", "-json", "-tags=integration", "testdir"},
		},
		"no -json arg": {
			opts: &options{
				args: []string{"-timeout=2m", "./pkg"},
			},
			expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg"},
		},
		"with -json arg": {
			opts: &options{
				args: []string{"-json", "-timeout=2m", "./pkg"},
			},
			expected: []string{"go", "test", "-json", "-timeout=2m", "./pkg"},
		},
		"raw command, with rerunOpts": {
			opts: &options{
				rawCommand: true,
				args:       []string{"./script", "-test.timeout=20m"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"./script", "-test.timeout=20m", "-run=TestOne|TestTwo", "./fails"},
		},
		"no args, with rerunOpts": {
			opts: &options{},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
		},
		"TEST_DIRECTORY env var, no args, with rerunOpts": {
			opts: &options{},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			env: []string{"TEST_DIRECTORY=testdir"},
			// TEST_DIRECTORY should be overridden by rerun opts
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "./fails"},
		},
		"TEST_DIRECTORY env var, with args, with rerunOpts": {
			opts: &options{
				args: []string{"-tags=integration"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			env:      []string{"TEST_DIRECTORY=testdir"},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-tags=integration", "./fails"},
		},
		"no -json arg, with rerunOpts": {
			opts: &options{
				args: []string{"-timeout=2m", "./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-timeout=2m", "./fails"},
		},
		"with -json arg, with rerunOpts": {
			opts: &options{
				args: []string{"-json", "-timeout=2m", "./pkg"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-run=TestOne|TestTwo", "-json", "-timeout=2m", "./fails"},
		},
		"multiple path args, with rerunOpts": {
			opts: &options{
				args: []string{"-timeout=2m", "./pkg1", "./pkg2", "./pkg3"},
			},
			rerunOpts: rerunOpts{
				runFlag: "-run=TestOne|TestTwo",
				pkg:     "./fails",
			},
			expected: []string{"go", "test", "-json", "-run=TestOne|TestTwo", "-timeout=2m", "./fails"},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

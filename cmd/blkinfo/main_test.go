// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeDev is a devFD that reports a fixed fd and records Close.
type fakeDev struct{ closed bool }

func (f *fakeDev) Fd() uintptr  { return 7 }
func (f *fakeDev) Close() error { f.closed = true; return nil }

// snapshot captures and restores all seams.
func snapshot() func() {
	o := openDevice
	a, b, c, d, e := getSize64, getSize, getSectorSize, getPhysBlockSize, getBlockSize
	g, h, i := getIOMin, getIOOpt, getReadOnly
	so, se := stdout, stderr
	return func() {
		openDevice = o
		getSize64, getSize, getSectorSize, getPhysBlockSize, getBlockSize = a, b, c, d, e
		getIOMin, getIOOpt, getReadOnly = g, h, i
		stdout, stderr = so, se
	}
}

// installOK wires every blk seam to a successful canned value.
func installOK() {
	getSize64 = func(int) (uint64, error) { return 1073741824, nil }
	getSize = func(int) (uint64, error) { return 2097152, nil }
	getSectorSize = func(int) (int, error) { return 512, nil }
	getPhysBlockSize = func(int) (int, error) { return 512, nil }
	getBlockSize = func(int) (int, error) { return 4096, nil }
	getIOMin = func(int) (int, error) { return 512, nil }
	getIOOpt = func(int) (int, error) { return 0, nil }
	getReadOnly = func(int) (bool, error) { return false, nil }
}

func TestRunHappyPath(t *testing.T) {
	defer snapshot()()
	dev := &fakeDev{}
	openDevice = func(string) (devFD, error) { return dev, nil }
	installOK()
	var out bytes.Buffer
	stdout = &out

	if rc := run([]string{"blkinfo", "/dev/loop0"}); rc != 0 {
		t.Fatalf("run rc = %d, want 0", rc)
	}
	if !dev.closed {
		t.Error("device not closed")
	}
	for _, want := range []string{"1073741824 bytes", "2097152 sectors", "512 (logical)", "4096 (soft)", "read-only:     false"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q\n%s", want, out.String())
		}
	}
}

func TestRunUsage(t *testing.T) {
	defer snapshot()()
	var errOut bytes.Buffer
	stderr = &errOut
	if rc := run([]string{"blkinfo"}); rc != 2 {
		t.Errorf("run with no arg rc = %d, want 2", rc)
	}
	if !strings.Contains(errOut.String(), "usage:") {
		t.Errorf("missing usage line: %s", errOut.String())
	}
}

func TestRunOpenError(t *testing.T) {
	defer snapshot()()
	openDevice = func(string) (devFD, error) { return nil, errors.New("boom") }
	var errOut bytes.Buffer
	stderr = &errOut
	if rc := run([]string{"blkinfo", "/dev/nope"}); rc != 1 {
		t.Errorf("rc = %d, want 1", rc)
	}
}

// TestRunIoctlErrors walks each query seam returning an error so every fail()
// branch in run() executes.
func TestRunIoctlErrors(t *testing.T) {
	boom := errors.New("boom")
	seams := []func(){
		func() { getSize64 = func(int) (uint64, error) { return 0, boom } },
		func() { getSize = func(int) (uint64, error) { return 0, boom } },
		func() { getSectorSize = func(int) (int, error) { return 0, boom } },
		func() { getPhysBlockSize = func(int) (int, error) { return 0, boom } },
		func() { getBlockSize = func(int) (int, error) { return 0, boom } },
		func() { getIOMin = func(int) (int, error) { return 0, boom } },
		func() { getIOOpt = func(int) (int, error) { return 0, boom } },
		func() { getReadOnly = func(int) (bool, error) { return false, boom } },
	}
	for i, breakOne := range seams {
		func() {
			defer snapshot()()
			openDevice = func(string) (devFD, error) { return &fakeDev{}, nil }
			installOK()
			breakOne()
			var errOut bytes.Buffer
			stderr = &errOut
			if rc := run([]string{"blkinfo", "/dev/loop0"}); rc != 1 {
				t.Errorf("seam %d: rc = %d, want 1", i, rc)
			}
		}()
	}
}

// TestMainInvokesRun drives the thin main() wrapper through the osExit seam.
func TestMainInvokesRun(t *testing.T) {
	defer snapshot()()
	origExit, origArgs := osExit, os.Args
	defer func() { osExit, os.Args = origExit, origArgs }()

	var code int
	osExit = func(c int) { code = c }
	os.Args = []string{"blkinfo"} // no device arg -> usage -> rc 2
	var errOut bytes.Buffer
	stderr = &errOut

	main()
	if code != 2 {
		t.Errorf("main() exit code = %d, want 2", code)
	}
}

func TestProgName(t *testing.T) {
	if progName(nil) != "blkinfo" {
		t.Error("progName(nil) should default to blkinfo")
	}
	if progName([]string{"x"}) != "x" {
		t.Error("progName should return argv[0]")
	}
}

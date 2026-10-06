// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

package blk

import (
	"runtime"
	"testing"
)

// kernelIOC re-encodes a request number written in the asm-generic _IOC layout
// (2 dir bits, 14 size bits, NONE=0 WRITE=1 READ=2) into the layout of the
// architecture the test runs on. The request table in abi_test.go is
// written once, in the asm-generic layout that the C preprocessor produced
// on x86-64; on powerpc and mips the kernel expects them re-encoded per
// arch/{powerpc,mips}/include/uapi/asm/ioctl.h (3 dir bits, 13 size bits,
// NONE=1 READ=2 WRITE=4).
//
// It deliberately does not use the package's _IOC_* constants, so a wrong layout
// in the package cannot agree with itself here; TestIoctlAgainstXSys checks this
// function against golang.org/x/sys/unix as well.
func kernelIOC(generic uintptr) uintptr {
	switch runtime.GOARCH {
	case "ppc64", "ppc64le", "mips", "mipsle", "mips64", "mips64le":
	default:
		return generic
	}
	dir := (generic >> 30) & 0x3
	size := (generic >> 16) & 0x3fff
	low := generic & 0xffff // type<<8 | nr: the same in both layouts
	var d uintptr
	if dir == 0 {
		d = 1 // _IOC_NONE
	}
	if dir&1 != 0 {
		d |= 4 // _IOC_WRITE
	}
	if dir&2 != 0 {
		d |= 2 // _IOC_READ
	}
	return d<<29 | size<<16 | low
}

// TestIocLayoutFillsTheWord checks that the per-architecture layout selected by
// build tag accounts for all 32 bits of the request word: 8 nr + 8 type + size
// + dir. A layout file with mismatched widths would fail here on its own lane.
func TestIocLayoutFillsTheWord(t *testing.T) {
	if got := _IOC_DIRSHIFT + _IOC_DIRBITS; got != 32 {
		t.Errorf("_IOC_DIRSHIFT+_IOC_DIRBITS = %d, want 32 (size bits %d, dir bits %d)", got, _IOC_SIZEBITS, _IOC_DIRBITS)
	}
}

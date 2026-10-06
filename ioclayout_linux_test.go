// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

package blk

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TestIoctlAgainstXSys judges this package's request numbers and _IOC helpers
// against a source that does not come from this package: golang.org/x/sys/unix,
// whose zerrors_linux_<arch>.go constants are generated per architecture from
// the kernel headers by a C compiler. The table in abi_test.go is written in
// the asm-generic layout, so an encoding that ignored powerpc's and mips' own
// layout agreed with it on every lane while the kernel answered ENOTTY.
//
// Every BLK* request that x/sys also defines is checked, then one helper call
// of each direction. BLKPG, BLKGETZONESZ and BLKGETNRZONES are not in x/sys;
// they go through the same helpers. It is a pure computation, so it runs under
// -test.short on the emulated lanes.
func TestIoctlAgainstXSys(t *testing.T) {
	sizeofLong := unsafe.Sizeof(uintptr(0)) // C long and size_t: pointer-sized on every Linux ABI Go has
	for _, c := range []struct {
		name      string
		got, want uintptr
	}{
		{"BLKROSET", BLKROSET, unix.BLKROSET},
		{"BLKROGET", BLKROGET, unix.BLKROGET},
		{"BLKRRPART", BLKRRPART, unix.BLKRRPART},
		{"BLKGETSIZE", BLKGETSIZE, unix.BLKGETSIZE},
		{"BLKFLSBUF", BLKFLSBUF, unix.BLKFLSBUF},
		{"BLKSSZGET", BLKSSZGET, unix.BLKSSZGET},
		{"BLKBSZGET", BLKBSZGET, unix.BLKBSZGET},
		{"BLKBSZSET", BLKBSZSET, unix.BLKBSZSET},
		{"BLKGETSIZE64", BLKGETSIZE64, unix.BLKGETSIZE64},
		{"BLKIOMIN", BLKIOMIN, unix.BLKIOMIN},
		{"BLKIOOPT", BLKIOOPT, unix.BLKIOOPT},
		{"BLKALIGNOFF", BLKALIGNOFF, unix.BLKALIGNOFF},
		{"BLKPBSZGET", BLKPBSZGET, unix.BLKPBSZGET},
		{"BLKDISCARDZEROES", BLKDISCARDZEROES, unix.BLKDISCARDZEROES},
		{"BLKDISCARD", BLKDISCARD, unix.BLKDISCARD},
		{"BLKSECDISCARD", BLKSECDISCARD, unix.BLKSECDISCARD},
		{"BLKZEROOUT", BLKZEROOUT, unix.BLKZEROOUT},

		{"_IO(0x12, 97) helper", io(blkMagic, 97), unix.BLKFLSBUF},
		{"_IOR(0x12, 114, size_t) helper", ior(blkMagic, 114, sizeofLong), unix.BLKGETSIZE64},
		{"_IOW(0x94, 9, int) FICLONE", iow(0x94, 9, 4), unix.FICLONE},
		{"_IOW('f', 2, long) FS_IOC_SETFLAGS", iow('f', 2, sizeofLong), unix.FS_IOC_SETFLAGS},
		{"_IOWR(0x94, 54, 24) FIDEDUPERANGE", ioc(_IOC_READ|_IOC_WRITE, 0x94, 54, 24), unix.FIDEDUPERANGE},

		// The test's own re-encoder must agree with x/sys too, or the pinned
		// table it feeds would be judged by something unchecked.
		{"kernelIOC(_IO BLKFLSBUF)", kernelIOC(0x1261), unix.BLKFLSBUF},
		{"kernelIOC(_IOW FICLONE)", kernelIOC(0x40049409), unix.FICLONE},
		{"kernelIOC(_IOWR FIDEDUPERANGE)", kernelIOC(0xc0189436), unix.FIDEDUPERANGE},
	} {
		if c.got != c.want {
			t.Errorf("%s = %#x, want %#x (x/sys/unix)", c.name, c.got, c.want)
		}
	}
}

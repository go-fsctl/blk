// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

//go:build linux

package blk

import (
	"os"
	"testing"
)

// requireDev opens the scratch block device named by the BLK_DEV environment
// variable, skipping the test if it is unset or the process is not root. Every
// BLK*/BLKPG ioctl that mutates state needs CAP_SYS_ADMIN; the read-only
// queries technically do not, but they still need a real block device, so we
// gate the whole integration suite on root + BLK_DEV. The returned fd is closed
// via t.Cleanup.
func requireDev(t *testing.T) int {
	t.Helper()
	dev := os.Getenv("BLK_DEV")
	if dev == "" {
		t.Skip("skipping: set BLK_DEV to a scratch block device (e.g. a loop device)")
	}
	if os.Geteuid() != 0 {
		t.Skip("skipping: block-device ioctls require root")
	}
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open %s: %v", dev, err)
	}
	t.Cleanup(func() { f.Close() })
	return int(f.Fd())
}

// TestQueries exercises the read-only geometry/size queries against the real
// device and sanity-checks their relationships.
func TestQueries(t *testing.T) {
	fd := requireDev(t)

	size64, err := GetSize64(fd)
	if err != nil {
		t.Fatalf("GetSize64: %v", err)
	}
	if size64 == 0 {
		t.Fatalf("GetSize64 = 0, want > 0")
	}
	t.Logf("GetSize64 = %d bytes", size64)

	sects, err := GetSize(fd)
	if err != nil {
		t.Fatalf("GetSize: %v", err)
	}
	if sects*512 != size64 {
		t.Errorf("GetSize*512 = %d, want %d (GetSize64)", sects*512, size64)
	}

	ssz, err := GetSectorSize(fd)
	if err != nil {
		t.Fatalf("GetSectorSize: %v", err)
	}
	pbsz, err := GetPhysBlockSize(fd)
	if err != nil {
		t.Fatalf("GetPhysBlockSize: %v", err)
	}
	bsz, err := GetBlockSize(fd)
	if err != nil {
		t.Fatalf("GetBlockSize: %v", err)
	}
	t.Logf("sector=%d phys=%d block=%d", ssz, pbsz, bsz)
	if ssz <= 0 || pbsz <= 0 || bsz <= 0 {
		t.Errorf("non-positive sizes: sector=%d phys=%d block=%d", ssz, pbsz, bsz)
	}

	for _, c := range []struct {
		name string
		fn   func(int) (int, error)
	}{
		{"GetIOMin", GetIOMin},
		{"GetIOOpt", GetIOOpt},
		{"GetAlignmentOffset", GetAlignmentOffset},
	} {
		if _, err := c.fn(fd); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if _, err := GetDiscardZeroes(fd); err != nil {
		t.Errorf("GetDiscardZeroes: %v", err)
	}
}

// TestReadOnlyToggle flips the read-only flag and reads it back, restoring the
// original state afterwards.
func TestReadOnlyToggle(t *testing.T) {
	fd := requireDev(t)

	orig, err := GetReadOnly(fd)
	if err != nil {
		t.Fatalf("GetReadOnly: %v", err)
	}
	t.Cleanup(func() { _ = SetReadOnly(fd, orig) })

	if err := SetReadOnly(fd, true); err != nil {
		t.Fatalf("SetReadOnly(true): %v", err)
	}
	if ro, err := GetReadOnly(fd); err != nil || !ro {
		t.Errorf("GetReadOnly after set true = %v, %v; want true, nil", ro, err)
	}
	if err := SetReadOnly(fd, false); err != nil {
		t.Fatalf("SetReadOnly(false): %v", err)
	}
	if ro, err := GetReadOnly(fd); err != nil || ro {
		t.Errorf("GetReadOnly after set false = %v, %v; want false, nil", ro, err)
	}
}

// TestZeroOutAndDiscard writes a non-zero pattern to the start of the device,
// zeroes it with BLKZEROOUT, verifies the read-back is zero, then issues a
// BLKDISCARD over the same range (which must not error).
func TestZeroOutAndDiscard(t *testing.T) {
	fd := requireDev(t)

	ssz, err := GetSectorSize(fd)
	if err != nil {
		t.Fatalf("GetSectorSize: %v", err)
	}
	n := ssz
	if n < 512 {
		n = 512
	}

	// Make sure it is writable for this test.
	if err := SetReadOnly(fd, false); err != nil {
		t.Fatalf("SetReadOnly(false): %v", err)
	}

	f := os.NewFile(uintptr(fd), "blkdev")
	pattern := make([]byte, n)
	for i := range pattern {
		pattern[i] = 0xff
	}
	if _, err := f.WriteAt(pattern, 0); err != nil {
		t.Fatalf("write 0xff: %v", err)
	}
	if err := f.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if err := ZeroOut(fd, 0, uint64(n)); err != nil {
		t.Fatalf("ZeroOut: %v", err)
	}
	if err := FlushBuf(fd); err != nil {
		t.Fatalf("FlushBuf: %v", err)
	}

	got := make([]byte, n)
	if _, err := f.ReadAt(got, 0); err != nil {
		t.Fatalf("read back: %v", err)
	}
	for i, b := range got {
		if b != 0 {
			t.Fatalf("byte %d = %#x after ZeroOut, want 0", i, b)
		}
	}

	// Discard over the same range must not error (effect is advisory).
	if err := Discard(fd, 0, uint64(n)); err != nil {
		t.Errorf("Discard: %v", err)
	}
}

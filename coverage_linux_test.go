// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

//go:build linux

package blk

import (
	"errors"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// These tests drive every branch of blk_linux.go through the ioctlPtr seam,
// fault-injecting both success (with a planted return value) and an errno,
// without needing root or a real block device. The root-only
// integration_linux_test.go exercises the genuine ioctls end to end.

// withIoctl swaps the ioctlPtr seam for fn for the duration of body, then
// restores it.
func withIoctl(fn func(fd int, req uintptr, arg unsafe.Pointer) error, body func()) {
	orig := ioctlPtr
	ioctlPtr = fn
	defer func() { ioctlPtr = orig }()
	body()
}

var errInjected = unix.EIO

// okWriting returns a seam that writes the given bytes into the argument the
// caller passed (simulating a kernel that fills in an out-parameter) and
// returns success. plant is interpreted per request width by the caller's type.
func okWriting(plant func(arg unsafe.Pointer)) func(int, uintptr, unsafe.Pointer) error {
	return func(_ int, _ uintptr, arg unsafe.Pointer) error {
		if plant != nil && arg != nil {
			plant(arg)
		}
		return nil
	}
}

func failSeam(_ int, _ uintptr, _ unsafe.Pointer) error { return errInjected }

// TestGetSize64 covers success and error.
func TestGetSize64(t *testing.T) {
	withIoctl(okWriting(func(p unsafe.Pointer) { *(*uint64)(p) = 1073741824 }), func() {
		v, err := GetSize64(3)
		if err != nil || v != 1073741824 {
			t.Errorf("GetSize64 = %d, %v; want 1073741824, nil", v, err)
		}
	})
	withIoctl(failSeam, func() {
		if _, err := GetSize64(3); !errors.Is(err, errInjected) {
			t.Errorf("GetSize64 err = %v, want EIO", err)
		}
	})
}

// TestScalarGetters covers every Get* that returns an int/bool/uint via an
// out-parameter, for both success and the errno branch.
func TestScalarGetters(t *testing.T) {
	type getter struct {
		name  string
		call  func(int) (int64, error) // normalized return
		plant func(unsafe.Pointer)
		want  int64
	}
	getters := []getter{
		{"GetSize", func(fd int) (int64, error) { v, e := GetSize(fd); return int64(v), e },
			func(p unsafe.Pointer) { *(*uint)(p) = 2097152 }, 2097152},
		{"GetBlockSize", func(fd int) (int64, error) { v, e := GetBlockSize(fd); return int64(v), e },
			func(p unsafe.Pointer) { *(*uint)(p) = 4096 }, 4096},
		{"GetSectorSize", func(fd int) (int64, error) { v, e := GetSectorSize(fd); return int64(v), e },
			func(p unsafe.Pointer) { *(*int32)(p) = 512 }, 512},
		{"GetPhysBlockSize", func(fd int) (int64, error) { v, e := GetPhysBlockSize(fd); return int64(v), e },
			func(p unsafe.Pointer) { *(*uint32)(p) = 4096 }, 4096},
		{"GetIOMin", func(fd int) (int64, error) { v, e := GetIOMin(fd); return int64(v), e },
			func(p unsafe.Pointer) { *(*uint32)(p) = 512 }, 512},
		{"GetIOOpt", func(fd int) (int64, error) { v, e := GetIOOpt(fd); return int64(v), e },
			func(p unsafe.Pointer) { *(*uint32)(p) = 0 }, 0},
		{"GetAlignmentOffset", func(fd int) (int64, error) { v, e := GetAlignmentOffset(fd); return int64(v), e },
			func(p unsafe.Pointer) { *(*int32)(p) = 0 }, 0},
	}
	for _, g := range getters {
		withIoctl(okWriting(g.plant), func() {
			got, err := g.call(3)
			if err != nil || got != g.want {
				t.Errorf("%s = %d, %v; want %d, nil", g.name, got, err, g.want)
			}
		})
		withIoctl(failSeam, func() {
			if _, err := g.call(3); !errors.Is(err, errInjected) {
				t.Errorf("%s err = %v, want EIO", g.name, err)
			}
		})
	}
}

// TestBoolGetters covers GetDiscardZeroes and GetReadOnly.
func TestBoolGetters(t *testing.T) {
	for _, g := range []struct {
		name string
		call func(int) (bool, error)
		set  func(unsafe.Pointer)
		want bool
	}{
		{"GetDiscardZeroes", GetDiscardZeroes, func(p unsafe.Pointer) { *(*uint32)(p) = 1 }, true},
		{"GetReadOnly", GetReadOnly, func(p unsafe.Pointer) { *(*int32)(p) = 1 }, true},
		{"GetReadOnlyFalse", GetReadOnly, func(p unsafe.Pointer) { *(*int32)(p) = 0 }, false},
	} {
		withIoctl(okWriting(g.set), func() {
			got, err := g.call(3)
			if err != nil || got != g.want {
				t.Errorf("%s = %v, %v; want %v, nil", g.name, got, err, g.want)
			}
		})
		withIoctl(failSeam, func() {
			if _, err := g.call(3); !errors.Is(err, errInjected) {
				t.Errorf("%s err = %v, want EIO", g.name, err)
			}
		})
	}
}

// TestSetters covers the mutating scalar ioctls: SetBlockSize, SetReadOnly.
func TestSetters(t *testing.T) {
	withIoctl(okWriting(nil), func() {
		if err := SetBlockSize(3, 4096); err != nil {
			t.Errorf("SetBlockSize: %v", err)
		}
		if err := SetReadOnly(3, true); err != nil {
			t.Errorf("SetReadOnly(true): %v", err)
		}
		if err := SetReadOnly(3, false); err != nil {
			t.Errorf("SetReadOnly(false): %v", err)
		}
	})
	withIoctl(failSeam, func() {
		if err := SetBlockSize(3, 4096); !errors.Is(err, errInjected) {
			t.Errorf("SetBlockSize err = %v, want EIO", err)
		}
		if err := SetReadOnly(3, true); !errors.Is(err, errInjected) {
			t.Errorf("SetReadOnly err = %v, want EIO", err)
		}
	})
}

// TestRangeOps covers Discard, SecureDiscard, ZeroOut.
func TestRangeOps(t *testing.T) {
	for _, op := range []struct {
		name string
		call func(int, uint64, uint64) error
	}{
		{"Discard", Discard},
		{"SecureDiscard", SecureDiscard},
		{"ZeroOut", ZeroOut},
	} {
		withIoctl(okWriting(nil), func() {
			if err := op.call(3, 0, 4096); err != nil {
				t.Errorf("%s: %v", op.name, err)
			}
		})
		withIoctl(failSeam, func() {
			if err := op.call(3, 0, 4096); !errors.Is(err, errInjected) {
				t.Errorf("%s err = %v, want EIO", op.name, err)
			}
		})
	}
}

// TestNoArgOps covers FlushBuf and RereadPartitionTable.
func TestNoArgOps(t *testing.T) {
	for _, op := range []struct {
		name string
		call func(int) error
	}{
		{"FlushBuf", FlushBuf},
		{"RereadPartitionTable", RereadPartitionTable},
	} {
		withIoctl(okWriting(nil), func() {
			if err := op.call(3); err != nil {
				t.Errorf("%s: %v", op.name, err)
			}
		})
		withIoctl(failSeam, func() {
			if err := op.call(3); !errors.Is(err, errInjected) {
				t.Errorf("%s err = %v, want EIO", op.name, err)
			}
		})
	}
}

// TestPartitionOps covers AddPartition, DelPartition, ResizePartition, and
// checks the blkpg argument the seam receives is well-formed.
func TestPartitionOps(t *testing.T) {
	check := func(wantOp int32) func(int, uintptr, unsafe.Pointer) error {
		return func(_ int, req uintptr, arg unsafe.Pointer) error {
			if req != BLKPG {
				t.Errorf("req = %#x, want BLKPG %#x", req, BLKPG)
			}
			a := (*blkpgIoctlArg)(arg)
			if a.Op != wantOp {
				t.Errorf("op = %d, want %d", a.Op, wantOp)
			}
			if a.DataLen != int32(unsafe.Sizeof(blkpgPartition{})) {
				t.Errorf("datalen = %d, want %d", a.DataLen, unsafe.Sizeof(blkpgPartition{}))
			}
			if a.Data == nil {
				t.Errorf("data pointer is nil")
			}
			return nil
		}
	}
	withIoctl(check(BLKPG_ADD_PARTITION), func() {
		if err := AddPartition(3, Partition{Number: 1, Start: 1 << 20, Length: 100 << 20}); err != nil {
			t.Errorf("AddPartition: %v", err)
		}
	})
	withIoctl(check(BLKPG_RESIZE_PARTITION), func() {
		if err := ResizePartition(3, Partition{Number: 1, Start: 1 << 20, Length: 200 << 20}); err != nil {
			t.Errorf("ResizePartition: %v", err)
		}
	})
	withIoctl(check(BLKPG_DEL_PARTITION), func() {
		if err := DelPartition(3, 1); err != nil {
			t.Errorf("DelPartition: %v", err)
		}
	})
	withIoctl(failSeam, func() {
		if err := AddPartition(3, Partition{Number: 1}); !errors.Is(err, errInjected) {
			t.Errorf("AddPartition err = %v, want EIO", err)
		}
		if err := DelPartition(3, 1); !errors.Is(err, errInjected) {
			t.Errorf("DelPartition err = %v, want EIO", err)
		}
		if err := ResizePartition(3, Partition{Number: 1}); !errors.Is(err, errInjected) {
			t.Errorf("ResizePartition err = %v, want EIO", err)
		}
	})
}

// TestZoneQueries covers GetZoneSize and GetNumZones.
func TestZoneQueries(t *testing.T) {
	withIoctl(okWriting(func(p unsafe.Pointer) { *(*uint32)(p) = 0 }), func() {
		if v, err := GetZoneSize(3); err != nil || v != 0 {
			t.Errorf("GetZoneSize = %d, %v; want 0, nil", v, err)
		}
		if v, err := GetNumZones(3); err != nil || v != 0 {
			t.Errorf("GetNumZones = %d, %v; want 0, nil", v, err)
		}
	})
	withIoctl(failSeam, func() {
		if _, err := GetZoneSize(3); !errors.Is(err, errInjected) {
			t.Errorf("GetZoneSize err = %v, want EIO", err)
		}
		if _, err := GetNumZones(3); !errors.Is(err, errInjected) {
			t.Errorf("GetNumZones err = %v, want EIO", err)
		}
	})
}

// TestRealIoctlPtrError drives the real syscall seam against an invalid fd so
// the errno-wrapping branch of realIoctlPtr executes without a fake.
func TestRealIoctlPtrError(t *testing.T) {
	var v uint64
	if err := realIoctlPtr(-1, BLKGETSIZE64, unsafe.Pointer(&v)); err == nil {
		t.Error("realIoctlPtr(-1) = nil, want errno")
	}
}

// TestRealIoctlPtrSuccess drives the success branch of realIoctlPtr without a
// block device or root by issuing a harmless TIOCINQ/FIONREAD on the read end
// of a pipe: it returns the number of bytes available (0) and, crucially,
// succeeds, so the errno==0 return path is exercised.
func TestRealIoctlPtrSuccess(t *testing.T) {
	var fds [2]int
	if err := unix.Pipe(fds[:]); err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer unix.Close(fds[0])
	defer unix.Close(fds[1])
	var n int32
	if err := realIoctlPtr(fds[0], unix.TIOCINQ, unsafe.Pointer(&n)); err != nil {
		t.Fatalf("realIoctlPtr TIOCINQ: %v", err)
	}
	if n != 0 {
		t.Errorf("TIOCINQ on empty pipe = %d, want 0", n)
	}
}

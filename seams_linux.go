// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

//go:build linux

package blk

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctlPtr is the single indirection seam over the raw SYS_IOCTL syscall this
// package drives. Every BLK*/BLKPG wrapper funnels through it with a pointer to
// its argument. It exists so the error branch of every kernel call — which only
// triggers on a real ioctl failure that is impractical to provoke against a
// live device — can be exercised deterministically by a fault-injecting fake in
// tests. Production code uses realIoctlPtr assigned here; tests swap the var,
// run, and restore it. The root-only integration test still drives the genuine
// ioctls for end-to-end confidence.
var ioctlPtr = realIoctlPtr

// realIoctlPtr issues ioctl(fd, req, arg) where arg is a pointer (or, for the
// _IO requests that pass a scalar by value such as BLKRRPART/BLKFLSBUF, nil).
// It returns the kernel's wrapped errno, or nil on success.
func realIoctlPtr(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

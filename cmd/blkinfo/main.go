// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl
//
// blkinfo is a live demonstration of github.com/go-fsctl/blk: given a block
// device path it opens the device and dumps its size, block/sector/physical
// sizes, I/O hints, and read-only state purely via BLK* ioctls (no cgo, no
// blockdev). It is the pure-Go analogue of "blockdev --report".
//
// Usage: blkinfo /dev/loop0
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/go-fsctl/blk"
)

// Seams over the OS and the blk package, overridable in tests. Production code
// uses the real implementations assigned here.
var (
	openDevice = func(path string) (devFD, error) { return os.OpenFile(path, os.O_RDONLY, 0) }

	getSize64        = blk.GetSize64
	getSize          = blk.GetSize
	getSectorSize    = blk.GetSectorSize
	getPhysBlockSize = blk.GetPhysBlockSize
	getBlockSize     = blk.GetBlockSize
	getIOMin         = blk.GetIOMin
	getIOOpt         = blk.GetIOOpt
	getReadOnly      = blk.GetReadOnly

	osExit            = os.Exit
	stdout io.Writer  = os.Stdout
	stderr io.Writer  = os.Stderr
)

// devFD is the slice of *os.File blkinfo needs: a file descriptor and Close.
type devFD interface {
	Fd() uintptr
	Close() error
}

func main() { osExit(run(os.Args)) }

func run(args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(stderr, "usage: %s /dev/blockdevice\n", progName(args))
		return 2
	}
	path := args[1]

	f, err := openDevice(path)
	if err != nil {
		fmt.Fprintf(stderr, "blkinfo: open %s: %v\n", path, err)
		return 1
	}
	defer f.Close()
	fd := int(f.Fd())

	size64, err := getSize64(fd)
	if err != nil {
		return fail("GetSize64", err)
	}
	sects, err := getSize(fd)
	if err != nil {
		return fail("GetSize", err)
	}
	ssz, err := getSectorSize(fd)
	if err != nil {
		return fail("GetSectorSize", err)
	}
	pbsz, err := getPhysBlockSize(fd)
	if err != nil {
		return fail("GetPhysBlockSize", err)
	}
	bsz, err := getBlockSize(fd)
	if err != nil {
		return fail("GetBlockSize", err)
	}
	iomin, err := getIOMin(fd)
	if err != nil {
		return fail("GetIOMin", err)
	}
	ioopt, err := getIOOpt(fd)
	if err != nil {
		return fail("GetIOOpt", err)
	}
	ro, err := getReadOnly(fd)
	if err != nil {
		return fail("GetReadOnly", err)
	}

	fmt.Fprintf(stdout, "device:        %s\n", path)
	fmt.Fprintf(stdout, "size:          %d bytes (%d sectors of 512)\n", size64, sects)
	fmt.Fprintf(stdout, "sector size:   %d (logical)\n", ssz)
	fmt.Fprintf(stdout, "phys block:    %d\n", pbsz)
	fmt.Fprintf(stdout, "block size:    %d (soft)\n", bsz)
	fmt.Fprintf(stdout, "io min/opt:    %d / %d\n", iomin, ioopt)
	fmt.Fprintf(stdout, "read-only:     %t\n", ro)
	return 0
}

func fail(what string, err error) int {
	fmt.Fprintf(stderr, "blkinfo: %s: %v\n", what, err)
	return 1
}

func progName(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "blkinfo"
}

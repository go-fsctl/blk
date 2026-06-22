// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

//go:build !linux

// Package blk wraps the generic Linux block-device ioctls (BLK* from
// linux/fs.h, BLKPG from linux/blkpg.h, and the zoned queries from
// linux/blkzoned.h). Those ioctls only exist on Linux; on other platforms every
// operation returns ErrUnsupported. The ABI definitions and ioctl-number
// derivation in abi.go remain available everywhere for testing and tooling.
package blk

// Partition describes a partition for the BLKPG add/resize operations. See the
// Linux build for field semantics.
type Partition struct {
	Number int
	Start  int64
	Length int64
}

// GetSize64 is unsupported off Linux.
func GetSize64(fd int) (uint64, error) { return 0, ErrUnsupported }

// GetSize is unsupported off Linux.
func GetSize(fd int) (uint64, error) { return 0, ErrUnsupported }

// GetBlockSize is unsupported off Linux.
func GetBlockSize(fd int) (int, error) { return 0, ErrUnsupported }

// SetBlockSize is unsupported off Linux.
func SetBlockSize(fd int, size int) error { return ErrUnsupported }

// GetSectorSize is unsupported off Linux.
func GetSectorSize(fd int) (int, error) { return 0, ErrUnsupported }

// GetPhysBlockSize is unsupported off Linux.
func GetPhysBlockSize(fd int) (int, error) { return 0, ErrUnsupported }

// GetIOMin is unsupported off Linux.
func GetIOMin(fd int) (int, error) { return 0, ErrUnsupported }

// GetIOOpt is unsupported off Linux.
func GetIOOpt(fd int) (int, error) { return 0, ErrUnsupported }

// GetAlignmentOffset is unsupported off Linux.
func GetAlignmentOffset(fd int) (int, error) { return 0, ErrUnsupported }

// GetDiscardZeroes is unsupported off Linux.
func GetDiscardZeroes(fd int) (bool, error) { return false, ErrUnsupported }

// GetReadOnly is unsupported off Linux.
func GetReadOnly(fd int) (bool, error) { return false, ErrUnsupported }

// SetReadOnly is unsupported off Linux.
func SetReadOnly(fd int, ro bool) error { return ErrUnsupported }

// Discard is unsupported off Linux.
func Discard(fd int, start, length uint64) error { return ErrUnsupported }

// SecureDiscard is unsupported off Linux.
func SecureDiscard(fd int, start, length uint64) error { return ErrUnsupported }

// ZeroOut is unsupported off Linux.
func ZeroOut(fd int, start, length uint64) error { return ErrUnsupported }

// FlushBuf is unsupported off Linux.
func FlushBuf(fd int) error { return ErrUnsupported }

// RereadPartitionTable is unsupported off Linux.
func RereadPartitionTable(fd int) error { return ErrUnsupported }

// AddPartition is unsupported off Linux.
func AddPartition(fd int, p Partition) error { return ErrUnsupported }

// DelPartition is unsupported off Linux.
func DelPartition(fd int, number int) error { return ErrUnsupported }

// ResizePartition is unsupported off Linux.
func ResizePartition(fd int, p Partition) error { return ErrUnsupported }

// GetZoneSize is unsupported off Linux.
func GetZoneSize(fd int) (uint32, error) { return 0, ErrUnsupported }

// GetNumZones is unsupported off Linux.
func GetNumZones(fd int) (uint32, error) { return 0, ErrUnsupported }

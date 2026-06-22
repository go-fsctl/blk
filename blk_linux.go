// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

//go:build linux

// Package blk wraps the generic Linux block-device ioctls — the BLK* requests
// from linux/fs.h, the BLKPG partition-table interface from linux/blkpg.h, and
// the zoned-device queries from linux/blkzoned.h — operating directly on an
// open block-device file descriptor with no cgo and no shelling out to
// blockdev, sgdisk, or partx.
//
// It is the generic block-device member of the go-fsctl family alongside
// github.com/go-fsctl/loop and github.com/go-fsctl/dm. Where those packages
// create block devices, this package inspects and manipulates any already-open
// block device: query its size and geometry, toggle its read-only flag,
// discard/zero byte ranges, flush its buffer cache, re-read its partition
// table, and add/delete/resize partitions.
//
// Querying calls (the Get* functions) need only a readable fd. The mutating
// calls — SetReadOnly, SetBlockSize, Discard, SecureDiscard, ZeroOut, FlushBuf,
// RereadPartitionTable, and the partition operations — require CAP_SYS_ADMIN
// (in practice, root) and, for the data operations, a writable fd. Every error
// wraps the underlying unix.Errno, so callers can errors.Is against the
// golang.org/x/sys/unix errno values.
package blk

import (
	"fmt"
	"unsafe"
)

// --- Size and geometry queries -------------------------------------------

// GetSize64 returns the size of the block device in bytes (BLKGETSIZE64). This
// is the modern, overflow-safe size query and the one to prefer.
func GetSize64(fd int) (uint64, error) {
	var v uint64
	if err := ioctlPtr(fd, BLKGETSIZE64, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKGETSIZE64: %w", err)
	}
	return v, nil
}

// GetSize returns the size of the block device in 512-byte sectors
// (BLKGETSIZE). The kernel writes a C long through the argument pointer; on a
// device larger than 2 TiB the value overflows a 32-bit long, so prefer
// GetSize64. The result is returned as a uint64 holding the long value.
func GetSize(fd int) (uint64, error) {
	var v uint  // matches C unsigned long on LP64/ILP32
	if err := ioctlPtr(fd, BLKGETSIZE, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKGETSIZE: %w", err)
	}
	return uint64(v), nil
}

// GetBlockSize returns the device's soft block size in bytes (BLKBSZGET). This
// is the block size used for buffered I/O, not necessarily the hardware sector
// size; see GetSectorSize and GetPhysBlockSize for those.
func GetBlockSize(fd int) (int, error) {
	var v uint  // BLKBSZGET reads a size_t (kernel: int promoted), positive
	if err := ioctlPtr(fd, BLKBSZGET, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKBSZGET: %w", err)
	}
	return int(v), nil
}

// SetBlockSize sets the device's soft block size in bytes (BLKBSZSET). The size
// must be a power of two between 512 and the page size; the kernel rejects
// other values with EINVAL. Requires CAP_SYS_ADMIN.
func SetBlockSize(fd int, size int) error {
	v := uint(size)
	if err := ioctlPtr(fd, BLKBSZSET, unsafe.Pointer(&v)); err != nil {
		return fmt.Errorf("blk: BLKBSZSET(%d): %w", size, err)
	}
	return nil
}

// GetSectorSize returns the device's logical sector size in bytes (BLKSSZGET) —
// the smallest addressable unit, typically 512 or 4096.
func GetSectorSize(fd int) (int, error) {
	var v int32 // BLKSSZGET writes a C int
	if err := ioctlPtr(fd, BLKSSZGET, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKSSZGET: %w", err)
	}
	return int(v), nil
}

// GetPhysBlockSize returns the device's physical block size in bytes
// (BLKPBSZGET) — the smallest unit the device can write without a
// read-modify-write, which on 512e drives differs from the logical sector size.
func GetPhysBlockSize(fd int) (int, error) {
	var v uint32 // BLKPBSZGET writes a C unsigned int
	if err := ioctlPtr(fd, BLKPBSZGET, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKPBSZGET: %w", err)
	}
	return int(v), nil
}

// GetIOMin returns the minimum I/O size in bytes the device prefers for
// efficient operation (BLKIOMIN).
func GetIOMin(fd int) (int, error) {
	var v uint32
	if err := ioctlPtr(fd, BLKIOMIN, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKIOMIN: %w", err)
	}
	return int(v), nil
}

// GetIOOpt returns the optimal I/O size in bytes the device reports (BLKIOOPT);
// 0 means the device exposes no preference.
func GetIOOpt(fd int) (int, error) {
	var v uint32
	if err := ioctlPtr(fd, BLKIOOPT, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKIOOPT: %w", err)
	}
	return int(v), nil
}

// GetAlignmentOffset returns the offset in bytes of the device's first logical
// block from its natural physical alignment (BLKALIGNOFF). It can be -1 when
// the alignment is unknown, hence the signed result.
func GetAlignmentOffset(fd int) (int, error) {
	var v int32 // BLKALIGNOFF writes a C int; -1 means "unknown"
	if err := ioctlPtr(fd, BLKALIGNOFF, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKALIGNOFF: %w", err)
	}
	return int(v), nil
}

// GetDiscardZeroes reports whether a discard on this device is guaranteed to
// return zeros on a subsequent read (BLKDISCARDZEROES). Note: this query is
// deprecated in the kernel and modern kernels always report 0; prefer ZeroOut
// when you need deterministic zeroing.
func GetDiscardZeroes(fd int) (bool, error) {
	var v uint32
	if err := ioctlPtr(fd, BLKDISCARDZEROES, unsafe.Pointer(&v)); err != nil {
		return false, fmt.Errorf("blk: BLKDISCARDZEROES: %w", err)
	}
	return v != 0, nil
}

// --- Read-only flag -------------------------------------------------------

// GetReadOnly reports whether the device is marked read-only (BLKROGET).
func GetReadOnly(fd int) (bool, error) {
	var v int32 // BLKROGET writes a C int (0 = read-write, 1 = read-only)
	if err := ioctlPtr(fd, BLKROGET, unsafe.Pointer(&v)); err != nil {
		return false, fmt.Errorf("blk: BLKROGET: %w", err)
	}
	return v != 0, nil
}

// SetReadOnly marks the device read-only (ro=true) or read-write (ro=false)
// (BLKROSET). Requires CAP_SYS_ADMIN. The kernel takes the value by reference
// to a C int.
func SetReadOnly(fd int, ro bool) error {
	var v int32
	if ro {
		v = 1
	}
	if err := ioctlPtr(fd, BLKROSET, unsafe.Pointer(&v)); err != nil {
		return fmt.Errorf("blk: BLKROSET(%t): %w", ro, err)
	}
	return nil
}

// --- Data operations ------------------------------------------------------

// blkRange is the u64[2] {start, length} byte-range argument shared by
// BLKDISCARD, BLKSECDISCARD, and BLKZEROOUT.
type blkRange [2]uint64

// Discard hints to the device that the byte range [start, start+length) is no
// longer needed (BLKDISCARD), e.g. a TRIM/UNMAP. start and length should be
// aligned to the device's discard granularity. Requires CAP_SYS_ADMIN.
func Discard(fd int, start, length uint64) error {
	r := blkRange{start, length}
	if err := ioctlPtr(fd, BLKDISCARD, unsafe.Pointer(&r)); err != nil {
		return fmt.Errorf("blk: BLKDISCARD(%d,%d): %w", start, length, err)
	}
	return nil
}

// SecureDiscard securely discards the byte range [start, start+length)
// (BLKSECDISCARD), instructing the device to erase the data rather than merely
// unmap it. Not all devices support it (ENOTSUPP/EOPNOTSUPP otherwise).
// Requires CAP_SYS_ADMIN.
func SecureDiscard(fd int, start, length uint64) error {
	r := blkRange{start, length}
	if err := ioctlPtr(fd, BLKSECDISCARD, unsafe.Pointer(&r)); err != nil {
		return fmt.Errorf("blk: BLKSECDISCARD(%d,%d): %w", start, length, err)
	}
	return nil
}

// ZeroOut writes zeros to the byte range [start, start+length) (BLKZEROOUT),
// using the device's hardware write-zeroes/offload path when available and
// falling back to an explicit zero write otherwise. Unlike Discard it
// guarantees the range reads back as zeros. Requires CAP_SYS_ADMIN.
func ZeroOut(fd int, start, length uint64) error {
	r := blkRange{start, length}
	if err := ioctlPtr(fd, BLKZEROOUT, unsafe.Pointer(&r)); err != nil {
		return fmt.Errorf("blk: BLKZEROOUT(%d,%d): %w", start, length, err)
	}
	return nil
}

// FlushBuf flushes and invalidates the device's buffer cache (BLKFLSBUF). It
// takes no argument. Requires CAP_SYS_ADMIN.
func FlushBuf(fd int) error {
	if err := ioctlPtr(fd, BLKFLSBUF, nil); err != nil {
		return fmt.Errorf("blk: BLKFLSBUF: %w", err)
	}
	return nil
}

// RereadPartitionTable asks the kernel to re-scan the device's partition table
// and recreate its partition device nodes (BLKRRPART). It fails with EBUSY if
// any partition is currently mounted or otherwise in use. Requires
// CAP_SYS_ADMIN.
func RereadPartitionTable(fd int) error {
	if err := ioctlPtr(fd, BLKRRPART, nil); err != nil {
		return fmt.Errorf("blk: BLKRRPART: %w", err)
	}
	return nil
}

// --- Partition table (BLKPG) ----------------------------------------------

// Partition describes a partition for the BLKPG add/resize operations. Start
// and Length are byte offsets/lengths within the whole-disk device; Number is
// the 1-based partition index (it becomes /dev/<disk>pN or /dev/<disk>N).
type Partition struct {
	// Number is the 1-based partition index (blkpg_partition.pno).
	Number int
	// Start is the partition's starting offset in bytes (blkpg_partition.start).
	Start int64
	// Length is the partition's length in bytes (blkpg_partition.length).
	Length int64
}

// blkpg issues a BLKPG ioctl with the given op against the whole-disk fd,
// passing part as the blkpg_partition payload.
func blkpg(fd int, op int32, part blkpgPartition) error {
	arg := blkpgIoctlArg{
		Op:      op,
		DataLen: int32(unsafe.Sizeof(part)),
		Data:    unsafe.Pointer(&part),
	}
	return ioctlPtr(fd, BLKPG, unsafe.Pointer(&arg))
}

// AddPartition adds partition p to the open whole-disk device fd without
// rewriting the on-disk partition table (BLKPG / BLKPG_ADD_PARTITION). The
// kernel creates the corresponding partition node (e.g. /dev/loop0p1). It is
// the in-kernel equivalent of "partx --add". Requires CAP_SYS_ADMIN.
func AddPartition(fd int, p Partition) error {
	part := blkpgPartition{
		Start:  p.Start,
		Length: p.Length,
		Pno:    int32(p.Number),
	}
	if err := blkpg(fd, BLKPG_ADD_PARTITION, part); err != nil {
		return fmt.Errorf("blk: BLKPG add partition %d: %w", p.Number, err)
	}
	return nil
}

// DelPartition removes the in-kernel partition with the given 1-based number
// from the open whole-disk device fd (BLKPG / BLKPG_DEL_PARTITION), the
// equivalent of "partx --delete". It does not modify the on-disk table.
// Requires CAP_SYS_ADMIN.
func DelPartition(fd int, number int) error {
	part := blkpgPartition{Pno: int32(number)}
	if err := blkpg(fd, BLKPG_DEL_PARTITION, part); err != nil {
		return fmt.Errorf("blk: BLKPG del partition %d: %w", number, err)
	}
	return nil
}

// ResizePartition updates the in-kernel start/length of an existing partition
// (BLKPG / BLKPG_RESIZE_PARTITION). Requires CAP_SYS_ADMIN.
func ResizePartition(fd int, p Partition) error {
	part := blkpgPartition{
		Start:  p.Start,
		Length: p.Length,
		Pno:    int32(p.Number),
	}
	if err := blkpg(fd, BLKPG_RESIZE_PARTITION, part); err != nil {
		return fmt.Errorf("blk: BLKPG resize partition %d: %w", p.Number, err)
	}
	return nil
}

// --- Zoned-device queries -------------------------------------------------

// GetZoneSize returns the zone size of a zoned block device in 512-byte sectors
// (BLKGETZONESZ). On a conventional (non-zoned) device the kernel returns 0.
func GetZoneSize(fd int) (uint32, error) {
	var v uint32
	if err := ioctlPtr(fd, BLKGETZONESZ, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKGETZONESZ: %w", err)
	}
	return v, nil
}

// GetNumZones returns the number of zones on a zoned block device
// (BLKGETNRZONES). On a conventional (non-zoned) device the kernel returns 0.
func GetNumZones(fd int) (uint32, error) {
	var v uint32
	if err := ioctlPtr(fd, BLKGETNRZONES, unsafe.Pointer(&v)); err != nil {
		return 0, fmt.Errorf("blk: BLKGETNRZONES: %w", err)
	}
	return v, nil
}

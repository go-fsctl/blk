// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

package blk

import "unsafe"

// The generic Linux block-device request numbers are defined in the kernel uapi
// headers linux/fs.h (BLK*), linux/blkpg.h (BLKPG), and linux/blkzoned.h
// (BLKGETZONESZ / BLKGETNRZONES). They all share the block ioctl magic byte
// 0x12.
//
// Most of them are PLAIN _IO(0x12, nr) requests: direction _IOC_NONE and size
// zero, so the wire value is simply (0x12 << 8) | nr. This is true even for
// requests that read or write data through a pointer argument (BLKGETSIZE,
// BLKSSZGET, BLKPBSZGET, BLKIOMIN, BLKIOOPT, BLKALIGNOFF, BLKDISCARDZEROES,
// BLKROGET, BLKROSET, BLKDISCARD, BLKSECDISCARD, BLKZEROOUT, BLKFLSBUF,
// BLKRRPART, BLKPG): the kernel historically declared them with _IO and the
// numbers are frozen for ABI stability.
//
// A handful DO encode a direction and a size:
//
//	#define BLKBSZGET    _IOR(0x12, 112, size_t)   // 0x80081270 on LP64
//	#define BLKBSZSET    _IOW(0x12, 113, size_t)   // 0x40081271 on LP64
//	#define BLKGETSIZE64 _IOR(0x12, 114, size_t)   // 0x80081272 on LP64
//	#define BLKGETZONESZ  _IOR(0x12, 132, __u32)   // 0x80041284
//	#define BLKGETNRZONES _IOR(0x12, 133, __u32)   // 0x80041285
//
// where size_t is 8 bytes on a 64-bit (LP64) kernel and __u32 is 4 bytes.
//
// We recompute every number in Go from the _IOC bit layout (the same one the
// kernel uses in asm-generic/ioctl.h) rather than hard-coding the hex, so the
// derivation is self-documenting and unit-testable; the expected hex values
// (verified against a C program compiled from the kernel headers) are pinned in
// abi_test.go.

// _IOC bit layout from asm-generic/ioctl.h. These are the values used by every
// mainstream architecture (the few exceptions — alpha, mips, powerpc, sparc —
// override the size/dir bit widths; this package targets the generic layout,
// which covers x86, arm, arm64, riscv64, loong64, s390x).
const (
	_IOC_NRBITS   = 8
	_IOC_TYPEBITS = 8
	_IOC_SIZEBITS = 14
	_IOC_DIRBITS  = 2

	_IOC_NRSHIFT   = 0
	_IOC_TYPESHIFT = _IOC_NRSHIFT + _IOC_NRBITS
	_IOC_SIZESHIFT = _IOC_TYPESHIFT + _IOC_TYPEBITS
	_IOC_DIRSHIFT  = _IOC_SIZESHIFT + _IOC_SIZEBITS

	_IOC_NONE  = 0
	_IOC_WRITE = 1
	_IOC_READ  = 2
)

// ioc assembles a request number exactly as the kernel's _IOC(dir,type,nr,size)
// macro does.
func ioc(dir, typ, nr, size uintptr) uintptr {
	return (dir << _IOC_DIRSHIFT) |
		(typ << _IOC_TYPESHIFT) |
		(nr << _IOC_NRSHIFT) |
		(size << _IOC_SIZESHIFT)
}

// io derives a plain _IO(type, nr): no direction, no size.
func io(typ, nr uintptr) uintptr { return ioc(_IOC_NONE, typ, nr, 0) }

// ior derives an _IOR(type, nr, size): kernel reads from the device into the
// user buffer (direction _IOC_READ), encoding the argument size.
func ior(typ, nr, size uintptr) uintptr { return ioc(_IOC_READ, typ, nr, size) }

// iow derives an _IOW(type, nr, size): kernel writes the user buffer to the
// device (direction _IOC_WRITE).
func iow(typ, nr, size uintptr) uintptr { return ioc(_IOC_WRITE, typ, nr, size) }

// blkMagic is the block ioctl type byte, 0x12, shared by all BLK* requests.
const blkMagic = 0x12

// sizeofSizeT is the size the kernel headers encode for size_t in
// BLKBSZGET/BLKBSZSET/BLKGETSIZE64. On every 64-bit platform this package
// targets, size_t is 8 bytes; the numbers are pinned for LP64 in abi_test.go.
const sizeofSizeT = 8

// sizeofU32 is the size encoded for __u32 in BLKGETZONESZ / BLKGETNRZONES.
const sizeofU32 = 4

// Block ioctl request numbers, derived from linux/fs.h, linux/blkpg.h, and
// linux/blkzoned.h. The trailing hex comment is the value on a 64-bit kernel.
var (
	// Read-only flag (BLKROGET/BLKROSET take an int*).
	BLKROSET = io(blkMagic, 93) // 0x125d
	BLKROGET = io(blkMagic, 94) // 0x125e

	// Partition-table maintenance.
	BLKRRPART = io(blkMagic, 95)  // 0x125f
	BLKFLSBUF = io(blkMagic, 97)  // 0x1261
	BLKPG     = io(blkMagic, 105) // 0x1269

	// Sizes and geometry.
	BLKGETSIZE       = io(blkMagic, 96)                // 0x1260  (long*, 512-byte sectors)
	BLKSSZGET        = io(blkMagic, 104)               // 0x1268  (int*, logical sector size)
	BLKBSZGET        = ior(blkMagic, 112, sizeofSizeT) // 0x80081270 (size_t*)
	BLKBSZSET        = iow(blkMagic, 113, sizeofSizeT) // 0x40081271 (size_t*)
	BLKGETSIZE64     = ior(blkMagic, 114, sizeofSizeT) // 0x80081272 (u64*, bytes)
	BLKIOMIN         = io(blkMagic, 120)               // 0x1278  (uint*)
	BLKIOOPT         = io(blkMagic, 121)               // 0x1279  (uint*)
	BLKALIGNOFF      = io(blkMagic, 122)               // 0x127a  (int*)
	BLKPBSZGET       = io(blkMagic, 123)               // 0x127b  (uint*, physical block size)
	BLKDISCARDZEROES = io(blkMagic, 124)               // 0x127c  (uint*)

	// Data operations.
	BLKDISCARD    = io(blkMagic, 119) // 0x1277  (u64[2] range)
	BLKSECDISCARD = io(blkMagic, 125) // 0x127d  (u64[2] range)
	BLKZEROOUT    = io(blkMagic, 127) // 0x127f  (u64[2] range)

	// Zoned-device queries.
	BLKGETZONESZ  = ior(blkMagic, 132, sizeofU32) // 0x80041284 (u32*, zone size in sectors)
	BLKGETNRZONES = ior(blkMagic, 133, sizeofU32) // 0x80041285 (u32*, number of zones)
)

// BLKPG subfunction codes from linux/blkpg.h (the blkpg_ioctl_arg.op field).
const (
	BLKPG_ADD_PARTITION    = 1
	BLKPG_DEL_PARTITION    = 2
	BLKPG_RESIZE_PARTITION = 3
)

// blkpgDevNameLen / blkpgVolNameLen are BLKPG_DEVNAMELTH / BLKPG_VOLNAMELTH
// from linux/blkpg.h: the (kernel-ignored) name buffers inside
// struct blkpg_partition.
const (
	blkpgDevNameLen = 64
	blkpgVolNameLen = 64
)

// blkpgPartition mirrors the kernel's struct blkpg_partition (linux/blkpg.h),
// the payload pointed to by blkpg_ioctl_arg.data for ADD/DEL/RESIZE.
//
//	struct blkpg_partition {
//		long long start;            // starting offset in bytes
//		long long length;           // length in bytes
//		int       pno;              // partition number
//		char      devname[BLKPG_DEVNAMELTH];
//		char      volname[BLKPG_VOLNAMELTH];
//	};
type blkpgPartition struct {
	Start   int64
	Length  int64
	Pno     int32
	DevName [blkpgDevNameLen]byte
	VolName [blkpgVolNameLen]byte
}

// blkpgIoctlArg mirrors the kernel's struct blkpg_ioctl_arg (linux/blkpg.h),
// the argument to the BLKPG ioctl. Data points at a blkpgPartition. On a 64-bit
// kernel the pointer is 8-byte aligned, so the struct is 24 bytes with 4 bytes
// of padding after datalen.
//
//	struct blkpg_ioctl_arg {
//		int   op;
//		int   flags;
//		int   datalen;
//		void *data;
//	};
type blkpgIoctlArg struct {
	Op      int32
	Flags   int32
	DataLen int32
	_       int32 // explicit padding to 8-byte-align Data on LP64
	Data    unsafe.Pointer
}

// blkZoneRange mirrors struct blk_zone_range (linux/blkzoned.h), kept here for
// completeness and ABI testing. It is the argument to the zone-management
// ioctls (reset/open/close/finish) which this package does not yet wrap.
//
//	struct blk_zone_range {
//		__u64 sector;       // starting sector of the range
//		__u64 nr_sectors;   // length of the range in sectors
//	};
type blkZoneRange struct {
	Sector    uint64
	NrSectors uint64
}

// abiSizeofBlkpgIoctlArg / abiSizeofBlkpgPartition / abiSizeofBlkZoneRange are
// recorded so abi_test.go can pin them against the kernel C sizeof() values on
// a 64-bit kernel.
var (
	abiSizeofBlkpgIoctlArg = unsafe.Sizeof(blkpgIoctlArg{})
	abiSizeofBlkpgPartition = unsafe.Sizeof(blkpgPartition{})
	abiSizeofBlkZoneRange   = unsafe.Sizeof(blkZoneRange{})
)

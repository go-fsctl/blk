// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

package blk

import (
	"testing"
	"unsafe"
)

// TestBlkIoctlNumbers pins the BLK* / BLKPG / zoned request numbers derived in
// abi.go to the values published by the kernel uapi headers on a 64-bit (LP64)
// kernel, where size_t is 8 bytes. These were verified by compiling a C program
// against linux/fs.h, linux/blkpg.h, and linux/blkzoned.h and printing each
// macro (see the package README / commit message).
func TestBlkIoctlNumbers(t *testing.T) {
	for _, c := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		// Plain _IO(0x12, nr): (0x12 << 8) | nr.
		{"BLKROSET", BLKROSET, 0x125d},
		{"BLKROGET", BLKROGET, 0x125e},
		{"BLKRRPART", BLKRRPART, 0x125f},
		{"BLKGETSIZE", BLKGETSIZE, 0x1260},
		{"BLKFLSBUF", BLKFLSBUF, 0x1261},
		{"BLKSSZGET", BLKSSZGET, 0x1268},
		{"BLKPG", BLKPG, 0x1269},
		{"BLKDISCARD", BLKDISCARD, 0x1277},
		{"BLKIOMIN", BLKIOMIN, 0x1278},
		{"BLKIOOPT", BLKIOOPT, 0x1279},
		{"BLKALIGNOFF", BLKALIGNOFF, 0x127a},
		{"BLKPBSZGET", BLKPBSZGET, 0x127b},
		{"BLKDISCARDZEROES", BLKDISCARDZEROES, 0x127c},
		{"BLKSECDISCARD", BLKSECDISCARD, 0x127d},
		{"BLKZEROOUT", BLKZEROOUT, 0x127f},

		// _IOR/_IOW with size_t (8 bytes): dir + size folded in.
		{"BLKBSZGET", BLKBSZGET, 0x80081270},
		{"BLKBSZSET", BLKBSZSET, 0x40081271},
		{"BLKGETSIZE64", BLKGETSIZE64, 0x80081272},

		// _IOR with __u32 (4 bytes).
		{"BLKGETZONESZ", BLKGETZONESZ, 0x80041284},
		{"BLKGETNRZONES", BLKGETNRZONES, 0x80041285},
	} {
		if c.got != c.want {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, c.want)
		}
	}
}

// TestIOCHelpers checks the _IOC derivation helpers independently of the
// concrete request numbers.
func TestIOCHelpers(t *testing.T) {
	if got := io(blkMagic, 119); got != 0x1277 {
		t.Errorf("io(0x12, 119) = %#x, want 0x1277", got)
	}
	if got := ior(blkMagic, 114, sizeofSizeT); got != 0x80081272 {
		t.Errorf("ior(0x12, 114, 8) = %#x, want 0x80081272", got)
	}
	if got := iow(blkMagic, 113, sizeofSizeT); got != 0x40081271 {
		t.Errorf("iow(0x12, 113, 8) = %#x, want 0x40081271", got)
	}
	if got := ior(blkMagic, 132, sizeofU32); got != 0x80041284 {
		t.Errorf("ior(0x12, 132, 4) = %#x, want 0x80041284", got)
	}
	// The direction and size bits must land where the kernel expects.
	if got := ioc(_IOC_READ, blkMagic, 114, sizeofSizeT); got != 0x80081272 {
		t.Errorf("ioc(READ, 0x12, 114, 8) = %#x, want 0x80081272", got)
	}
}

// TestBlkpgSubfunctions pins the BLKPG op codes to linux/blkpg.h.
func TestBlkpgSubfunctions(t *testing.T) {
	if BLKPG_ADD_PARTITION != 1 || BLKPG_DEL_PARTITION != 2 || BLKPG_RESIZE_PARTITION != 3 {
		t.Errorf("BLKPG ops = %d/%d/%d, want 1/2/3",
			BLKPG_ADD_PARTITION, BLKPG_DEL_PARTITION, BLKPG_RESIZE_PARTITION)
	}
}

// TestStructSizes pins the ioctl struct sizes to the C sizeof() values from
// linux/blkpg.h / linux/blkzoned.h on a 64-bit kernel: blkpg_ioctl_arg is 24
// bytes (3 ints + 4 pad + 8-byte pointer), blkpg_partition is 152 bytes
// (8 + 8 + 4 pad-to-4 + 64 + 64 = ... see offsets), blk_zone_range is 16 bytes.
func TestStructSizes(t *testing.T) {
	if got := unsafe.Sizeof(blkpgIoctlArg{}); got != 24 {
		t.Errorf("sizeof(blkpg_ioctl_arg) = %d, want 24", got)
	}
	if got := unsafe.Sizeof(blkpgPartition{}); got != 152 {
		t.Errorf("sizeof(blkpg_partition) = %d, want 152", got)
	}
	if got := unsafe.Sizeof(blkZoneRange{}); got != 16 {
		t.Errorf("sizeof(blk_zone_range) = %d, want 16", got)
	}
	// The recorded sizes used by integration code should agree.
	if abiSizeofBlkpgIoctlArg != 24 || abiSizeofBlkpgPartition != 152 || abiSizeofBlkZoneRange != 16 {
		t.Errorf("recorded sizes = %d/%d/%d, want 24/152/16",
			abiSizeofBlkpgIoctlArg, abiSizeofBlkpgPartition, abiSizeofBlkZoneRange)
	}
}

// TestStructOffsets pins the byte offsets of every field inside
// struct blkpg_ioctl_arg and struct blkpg_partition to their kernel ABI
// positions on a 64-bit kernel.
func TestStructOffsets(t *testing.T) {
	var a blkpgIoctlArg
	for _, c := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"op", unsafe.Offsetof(a.Op), 0},
		{"flags", unsafe.Offsetof(a.Flags), 4},
		{"datalen", unsafe.Offsetof(a.DataLen), 8},
		{"data", unsafe.Offsetof(a.Data), 16},
	} {
		if c.got != c.want {
			t.Errorf("offsetof(blkpg_ioctl_arg.%s) = %d, want %d", c.name, c.got, c.want)
		}
	}

	var p blkpgPartition
	for _, c := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"start", unsafe.Offsetof(p.Start), 0},
		{"length", unsafe.Offsetof(p.Length), 8},
		{"pno", unsafe.Offsetof(p.Pno), 16},
		{"devname", unsafe.Offsetof(p.DevName), 20},
		{"volname", unsafe.Offsetof(p.VolName), 84},
	} {
		if c.got != c.want {
			t.Errorf("offsetof(blkpg_partition.%s) = %d, want %d", c.name, c.got, c.want)
		}
	}
}

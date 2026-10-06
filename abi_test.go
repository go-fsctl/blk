// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

package blk

import (
	"runtime"
	"testing"
	"unsafe"
)

// TestBlkIoctlNumbers pins the BLK* / BLKPG / zoned request numbers derived in
// abi.go to the values published by the kernel uapi headers on a 64-bit (LP64)
// kernel, where size_t is 8 bytes. These were verified by compiling a C program
// against linux/fs.h, linux/blkpg.h, and linux/blkzoned.h and printing each
// macro (see the package README / commit message). They are written in the
// asm-generic _IOC layout; kernelIOC re-encodes them for powerpc and mips, and
// TestIoctlAgainstXSys checks them against an independent per-architecture
// source.
func TestBlkIoctlNumbers(t *testing.T) {
	const sizeT = unsafe.Sizeof(uintptr(0)) // size_t: pointer-sized on every Linux ABI Go has
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

		// _IOR/_IOW with size_t: dir + size folded in. size_t is 8 bytes on
		// a 64-bit kernel (0x80081270) and 4 on a 32-bit one (0x80041270).
		{"BLKBSZGET", BLKBSZGET, 0x80001270 | sizeT<<16},
		{"BLKBSZSET", BLKBSZSET, 0x40001271 | sizeT<<16},
		{"BLKGETSIZE64", BLKGETSIZE64, 0x80001272 | sizeT<<16},

		// _IOR with __u32 (4 bytes).
		{"BLKGETZONESZ", BLKGETZONESZ, 0x80041284},
		{"BLKGETNRZONES", BLKGETNRZONES, 0x80041285},
	} {
		if want := kernelIOC(c.want); c.got != want {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, want)
		}
	}
}

// TestIOCHelpers checks the _IOC derivation helpers independently of the
// concrete request numbers. The expected values are written in the asm-generic
// layout and re-encoded by kernelIOC for powerpc and mips.
func TestIOCHelpers(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want uintptr
	}{
		{"io(0x12, 119)", io(blkMagic, 119), 0x1277},
		{"ior(0x12, 114, 8)", ior(blkMagic, 114, 8), 0x80081272},
		{"iow(0x12, 113, 8)", iow(blkMagic, 113, 8), 0x40081271},
		{"ior(0x12, 132, 4)", ior(blkMagic, 132, sizeofU32), 0x80041284},
		// The direction and size bits must land where the kernel expects.
		{"ioc(READ, 0x12, 114, 8)", ioc(_IOC_READ, blkMagic, 114, 8), 0x80081272},
	} {
		if want := kernelIOC(c.want); c.got != want {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, want)
		}
	}
}

// TestBlkpgSubfunctions pins the BLKPG op codes to linux/blkpg.h.
func TestBlkpgSubfunctions(t *testing.T) {
	if BLKPG_ADD_PARTITION != 1 || BLKPG_DEL_PARTITION != 2 || BLKPG_RESIZE_PARTITION != 3 {
		t.Errorf("BLKPG ops = %d/%d/%d, want 1/2/3",
			BLKPG_ADD_PARTITION, BLKPG_DEL_PARTITION, BLKPG_RESIZE_PARTITION)
	}
}

// cLayout is the C layout of struct blkpg_ioctl_arg and struct blkpg_partition
// (include/uapi/linux/blkpg.h) on the architecture the test runs on, computed
// by hand from the C alignment rules rather than from the Go structs:
//
//	struct blkpg_ioctl_arg { int op; int flags; int datalen; void *data; };
//	struct blkpg_partition { long long start; long long length; int pno;
//	                         char devname[64]; char volname[64]; };
//
// blkpg_ioctl_arg: three ints end at 12. On LP64 the 8-byte pointer is 8-byte
// aligned, so data sits at 16 and the struct is 24; on ILP32 the 4-byte pointer
// follows at 12 and the struct is 16.
//
// blkpg_partition: start 0, length 8, pno 16, devname 20, volname 84, end 148.
// The struct is padded to the alignment of long long: 8 on every 64-bit ABI
// and on 32-bit arm (EABI), mips (o32) and powerpc, so 152; only i386 aligns
// long long to 4, so 148 there.
type cLayout struct {
	argSize, argData uintptr // sizeof(blkpg_ioctl_arg), offsetof(data)
	partSize         uintptr // sizeof(blkpg_partition)
}

func cLayoutFor(goarch string) cLayout {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return cLayout{argSize: 24, argData: 16, partSize: 152}
	}
	if goarch == "386" {
		return cLayout{argSize: 16, argData: 12, partSize: 148}
	}
	return cLayout{argSize: 16, argData: 12, partSize: 152} // arm, mips, mipsle
}

// kernel64PartSize is sizeof(struct blkpg_partition) in a 64-bit kernel. A
// 32-bit process running on a 64-bit kernel reaches compat_blkpg_ioctl
// (block/ioctl.c), which hands the user pointer to blkpg_do_ioctl, and that
// copies sizeof(struct blkpg_partition) -- the kernel's own, 152 bytes. So the
// Go payload must be at least 152 bytes even on i386, where the native C
// struct is 148, or the kernel reads past the end of it.
const kernel64PartSize = 152

// TestStructSizes pins the ioctl struct sizes to the C layout of the
// architecture the test runs on (see cLayout). Under the emulated 32-bit lanes
// this is what catches a Go struct laid out for LP64 only: an explicit pad
// before the pointer made blkpg_ioctl_arg 20 bytes where C has 16, and Go's
// 4-byte alignment of int64 on 32-bit made blkpg_partition 148 where arm and
// mips C have 152.
func TestStructSizes(t *testing.T) {
	c := cLayoutFor(runtime.GOARCH)
	if got := unsafe.Sizeof(blkpgIoctlArg{}); got != c.argSize {
		t.Errorf("sizeof(blkpg_ioctl_arg) = %d on %s, want %d (C layout)", got, runtime.GOARCH, c.argSize)
	}
	got := unsafe.Sizeof(blkpgPartition{})
	if got < c.partSize {
		t.Errorf("sizeof(blkpg_partition) = %d on %s, smaller than C's %d: the kernel copies past it", got, runtime.GOARCH, c.partSize)
	}
	if got != kernel64PartSize {
		t.Errorf("sizeof(blkpg_partition) = %d on %s, want %d (what a 64-bit kernel copies, compat path included)", got, runtime.GOARCH, kernel64PartSize)
	}
	if got := unsafe.Sizeof(blkZoneRange{}); got != 16 {
		t.Errorf("sizeof(blk_zone_range) = %d, want 16", got)
	}
	// The recorded sizes used by integration code should agree.
	if abiSizeofBlkpgIoctlArg != c.argSize || abiSizeofBlkpgPartition != kernel64PartSize || abiSizeofBlkZoneRange != 16 {
		t.Errorf("recorded sizes = %d/%d/%d, want %d/%d/16",
			abiSizeofBlkpgIoctlArg, abiSizeofBlkpgPartition, abiSizeofBlkZoneRange, c.argSize, kernel64PartSize)
	}
}

// TestCLayoutTable checks the hand-computed table itself on every lane, so a
// typo in it cannot hide behind the one architecture the test happens to run on.
func TestCLayoutTable(t *testing.T) {
	for _, c := range []struct {
		goarch string
		ptr    uintptr
		want   cLayout
	}{
		{"386", 4, cLayout{16, 12, 148}},
		{"arm", 4, cLayout{16, 12, 152}},
		{"mips", 4, cLayout{16, 12, 152}},
		{"mipsle", 4, cLayout{16, 12, 152}},
		{"amd64", 8, cLayout{24, 16, 152}},
		{"s390x", 8, cLayout{24, 16, 152}},
	} {
		if c.ptr != unsafe.Sizeof(uintptr(0)) {
			continue // cLayoutFor reads the pointer size of the running lane
		}
		if got := cLayoutFor(c.goarch); got != c.want {
			t.Errorf("cLayoutFor(%s) = %+v, want %+v", c.goarch, got, c.want)
		}
	}
}

// TestStructOffsets pins the byte offsets of every field inside
// struct blkpg_ioctl_arg and struct blkpg_partition to their C positions on
// the architecture the test runs on.
func TestStructOffsets(t *testing.T) {
	cl := cLayoutFor(runtime.GOARCH)
	var a blkpgIoctlArg
	for _, c := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"op", unsafe.Offsetof(a.Op), 0},
		{"flags", unsafe.Offsetof(a.Flags), 4},
		{"datalen", unsafe.Offsetof(a.DataLen), 8},
		{"data", unsafe.Offsetof(a.Data), cl.argData},
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

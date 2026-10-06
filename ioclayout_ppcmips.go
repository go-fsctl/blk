// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

//go:build ppc64 || ppc64le || mips || mipsle || mips64 || mips64le

package blk

// The powerpc and mips _IOC layout, from arch/powerpc/include/uapi/asm/ioctl.h
// and arch/mips/include/uapi/asm/ioctl.h (identical on this point): a 13-bit
// size field, a 3-bit direction field, and direction values that differ from
// asm-generic -- _IOC_NONE is 1, not 0, and _IOC_WRITE is 4, not 1. sparc and
// alpha use the same layout, but Go has no Linux port for either.
const (
	_IOC_SIZEBITS = 13
	_IOC_DIRBITS  = 3

	_IOC_NONE  = 1
	_IOC_READ  = 2
	_IOC_WRITE = 4
)

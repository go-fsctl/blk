// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

//go:build !ppc64 && !ppc64le && !mips && !mipsle && !mips64 && !mips64le

package blk

// The asm-generic _IOC layout (include/uapi/asm-generic/ioctl.h), used by
// every Linux architecture Go supports except powerpc and mips (see
// ioclayout_ppcmips.go).
const (
	_IOC_SIZEBITS = 14
	_IOC_DIRBITS  = 2

	_IOC_NONE  = 0
	_IOC_WRITE = 1
	_IOC_READ  = 2
)

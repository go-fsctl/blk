// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, go-fsctl

package blk

import "errors"

// ErrUnsupported is returned by every operation on non-Linux platforms, where
// the BLK*/BLKPG ioctls do not exist. It is declared in a platform-independent
// file so callers can reference blk.ErrUnsupported on any GOOS; on Linux it is
// never returned.
var ErrUnsupported = errors.New("blk: block-device ioctls are only supported on Linux")

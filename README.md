# go-fsctl/blk

[![Go Reference](https://pkg.go.dev/badge/github.com/go-fsctl/blk.svg)](https://pkg.go.dev/github.com/go-fsctl/blk)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![CI](https://github.com/go-fsctl/blk/actions/workflows/ci.yml/badge.svg)](https://github.com/go-fsctl/blk/actions/workflows/ci.yml)

Pure-Go generic Linux block-device ioctls: query a block device's size and
geometry, toggle its read-only flag, discard/zero ranges, flush its buffer
cache, re-read its partition table, and add/delete/resize partitions — all
directly through the kernel's `BLK*` and `BLKPG` ioctls on an open device fd,
with **no cgo** and **no shelling out** to `blockdev`, `sgdisk`, or `partx`.

This is the generic block-device member of the
[`go-fsctl`](https://github.com/go-fsctl) family, alongside
[`go-fsctl/loop`](https://github.com/go-fsctl/loop),
[`go-fsctl/dm`](https://github.com/go-fsctl/dm),
[`go-fsctl/zfs`](https://github.com/go-fsctl/zfs), and
[`go-fsctl/btrfs`](https://github.com/go-fsctl/btrfs). Where `loop` and `dm`
create block devices, this package operates on any already-open block device
(a real disk, a partition, a loop device, a device-mapper target, …).

## Status

The ABI numbers and structs are derived from the kernel uapi headers
`linux/fs.h`, `linux/blkpg.h`, and `linux/blkzoned.h`, recomputed in pure Go
from the `_IOC` bit layout, and pinned in the host-runnable `abi_test.go`; see
`abi.go`. Integration against a live kernel is cross-checked with `blockdev` and
`lsblk`/`partx` (see [Testing](#testing)).

## API

```go
import "github.com/go-fsctl/blk"

f, _ := os.OpenFile("/dev/loop0", os.O_RDONLY, 0)
fd := int(f.Fd())

// Sizes and geometry:
bytes,  _ := blk.GetSize64(fd)        // BLKGETSIZE64  (device size in bytes)
sects,  _ := blk.GetSize(fd)          // BLKGETSIZE    (size in 512-byte sectors)
ssz,    _ := blk.GetSectorSize(fd)    // BLKSSZGET     (logical sector size)
pbsz,   _ := blk.GetPhysBlockSize(fd) // BLKPBSZGET    (physical block size)
bsz,    _ := blk.GetBlockSize(fd)     // BLKBSZGET     (soft block size)

iomin,  _ := blk.GetIOMin(fd)         // BLKIOMIN
ioopt,  _ := blk.GetIOOpt(fd)         // BLKIOOPT
aoff,   _ := blk.GetAlignmentOffset(fd) // BLKALIGNOFF
dz,     _ := blk.GetDiscardZeroes(fd) // BLKDISCARDZEROES

// Read-only flag (needs CAP_SYS_ADMIN to set):
ro, _ := blk.GetReadOnly(fd)          // BLKROGET
_     = blk.SetReadOnly(fd, true)     // BLKROSET
_     = blk.SetBlockSize(fd, 4096)    // BLKBSZSET

// Data operations (privileged; ranges are byte offsets/lengths):
_ = blk.Discard(fd, start, length)        // BLKDISCARD
_ = blk.SecureDiscard(fd, start, length)  // BLKSECDISCARD
_ = blk.ZeroOut(fd, start, length)        // BLKZEROOUT
_ = blk.FlushBuf(fd)                      // BLKFLSBUF
_ = blk.RereadPartitionTable(fd)          // BLKRRPART

// Partition table via BLKPG:
_ = blk.AddPartition(fd, blk.Partition{Number: 1, Start: 1<<20, Length: 100<<20})
_ = blk.ResizePartition(fd, blk.Partition{Number: 1, Start: 1<<20, Length: 200<<20})
_ = blk.DelPartition(fd, 1)

// Zoned-device queries (return ErrNotZoned-style errno on conventional disks):
zsz, _ := blk.GetZoneSize(fd)   // BLKGETZONESZ  (zone size in 512-byte sectors)
nz,  _ := blk.GetNumZones(fd)   // BLKGETNRZONES
```

Every function takes an integer file descriptor for an open block device. The
querying calls (`Get*`) only need read access; the mutating calls (`SetReadOnly`,
`Discard`, `ZeroOut`, `RereadPartitionTable`, the `*Partition` calls) require
`CAP_SYS_ADMIN` (in practice, root) and a writable fd. On non-Linux platforms
every function returns `blk.ErrUnsupported`.

## BLK* ioctls used

| Function                 | ioctl              | header            | request (LP64) |
|--------------------------|--------------------|-------------------|----------------|
| `GetSize64`              | `BLKGETSIZE64`     | `linux/fs.h`      | `0x80081272`   |
| `GetSize`                | `BLKGETSIZE`       | `linux/fs.h`      | `0x1260`       |
| `GetBlockSize`           | `BLKBSZGET`        | `linux/fs.h`      | `0x80081270`   |
| `SetBlockSize`           | `BLKBSZSET`        | `linux/fs.h`      | `0x40081271`   |
| `GetSectorSize`          | `BLKSSZGET`        | `linux/fs.h`      | `0x1268`       |
| `GetPhysBlockSize`       | `BLKPBSZGET`       | `linux/fs.h`      | `0x127b`       |
| `GetIOMin`               | `BLKIOMIN`         | `linux/fs.h`      | `0x1278`       |
| `GetIOOpt`               | `BLKIOOPT`         | `linux/fs.h`      | `0x1279`       |
| `GetAlignmentOffset`     | `BLKALIGNOFF`      | `linux/fs.h`      | `0x127a`       |
| `GetDiscardZeroes`       | `BLKDISCARDZEROES` | `linux/fs.h`      | `0x127c`       |
| `GetReadOnly`            | `BLKROGET`         | `linux/fs.h`      | `0x125e`       |
| `SetReadOnly`            | `BLKROSET`         | `linux/fs.h`      | `0x125d`       |
| `Discard`                | `BLKDISCARD`       | `linux/fs.h`      | `0x1277`       |
| `SecureDiscard`          | `BLKSECDISCARD`    | `linux/fs.h`      | `0x127d`       |
| `ZeroOut`                | `BLKZEROOUT`       | `linux/fs.h`      | `0x127f`       |
| `FlushBuf`               | `BLKFLSBUF`        | `linux/fs.h`      | `0x1261`       |
| `RereadPartitionTable`   | `BLKRRPART`        | `linux/fs.h`      | `0x125f`       |
| `AddPartition`/`DelPartition`/`ResizePartition` | `BLKPG` | `linux/blkpg.h` | `0x1269` |
| `GetZoneSize`            | `BLKGETZONESZ`     | `linux/blkzoned.h`| `0x80041284`   |
| `GetNumZones`            | `BLKGETNRZONES`    | `linux/blkzoned.h`| `0x80041285`   |

## Testing

```sh
# Host-runnable unit tests (ioctl numbers, struct sizes/offsets):
GOWORK=off go test ./...

# Integration tests are gated on root + a block device. Point BLK_DEV at a
# scratch device (e.g. a loop device) and run as root on a Linux box:
sudo BLK_DEV=/dev/loop0 -E env "PATH=$PATH" go test -run Integration -v ./...
```

There is also a live demo binary, `cmd/blkinfo`, that opens a block device and
dumps its size, block/sector/physical sizes, and read-only state.

## License

BSD-3-Clause. See [LICENSE](LICENSE).

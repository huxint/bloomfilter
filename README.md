# bloomfilter

**English** | [简体中文](README.zh-CN.md)

A production-ready, general-purpose Go membership-test library: a classic
**Bloom filter** and a **counting Bloom filter** (with `Remove`), with binary
serialization and `mmap` persistence for billions-scale sets.

> Not safe for concurrent use — the caller synchronizes (like the built-in `map`).

## Install

```bash
go get github.com/huxint/bloomfilter
```

## Quick start

```go
f, _ := bloomfilter.New(1_000_000, 0.001) // 1M items, 0.1% false-positive rate
f.AddString("alice")

if !f.MightContainString("bob") {
    // definitely available — no database lookup needed
}
```

## How it works

A Bloom filter is a bit array plus *k* hash functions. It has **no false
negatives** (a "not present" answer is always correct) and a tunable
**false-positive** rate. That makes it ideal as a *negative cache*: most
"is this taken?" checks are answered in sub-microsecond time without touching
the authoritative store; only a small, bounded fraction fall back to a real query.

The counting variant stores a 4-bit counter per cell instead of a single bit, so
it additionally supports `Remove` (e.g. a username is released when an account is
deleted).

## API

| Function | Purpose |
|---|---|
| `New(n, p) (*BloomFilter, error)` | In-memory classic filter for `n` items at FP rate `p` |
| `NewCounting(n, p) (*CountingFilter, error)` | In-memory counting filter (supports `Remove`) |
| `CreateMmap(path, kind, n, p) (MmapFilter, error)` | Build a file-backed filter too large for RAM |
| `OpenMmap(path, readOnly) (MmapFilter, error)` | Map an existing filter file (`readOnly` → query-only) |
| `Save(f, path)` / `Load(path)` | Serialize to / from disk |

Filter methods: `Add`, `MightContain`, `AddString`, `MightContainString`,
`AddedCount`, `Params`, plus `Remove` (counting), `EstimateCardinality` /
`EstimateFalsePositiveRate` (classic). All implement `encoding.BinaryMarshaler`,
`io.WriterTo` / `io.ReaderFrom`; mmap filters also expose `Sync` / `Close`.

Hot-path methods (`Add` / `MightContain` / `Remove`) return no error and never
allocate. Construction and I/O return errors; a corrupt or truncated file is
reported as an error (`ErrBadMagic`, `ErrVersion`, `ErrCorrupt`, …), never a panic.

## Use cases (see `examples/`)

- **username** — username/email availability (negative cache + DB fallback + release via counting filter)
- **crawler** — seen-URL dedup that survives restarts via `Save`/`Load`
- **cacheguard** — cache-penetration protection: skip the DB for definitely-absent keys
- **blocklist** — weak-password/malicious-URL blocklist via read-only `mmap` of a prebuilt file

## Persistence

```go
bloomfilter.Save(f, "f.blmf")      // serialize to disk
g, _ := bloomfilter.Load("f.blmf") // reload into memory

// Or mmap a file too large to rebuild on restart:
mf, _ := bloomfilter.CreateMmap("big.blmf", bloomfilter.KindBloom, 10_000_000_000, 0.001)
defer mf.Close()
ro, _ := bloomfilter.OpenMmap("big.blmf", true) // read-only, query-serving
defer ro.Close()
```

`mmap` uses `golang.org/x/sys` — the only dependency outside the standard library.

## Memory sizing (classic Bloom; counting Bloom is 4×)

Bits per item ≈ `-ln p / (ln 2)²`.

| n | p=1% (k≈7) | p=0.1% (k≈10) | p=0.01% (k≈13) |
|---|---|---|---|
| 1e8 (100M) | ≈ 120 MB | ≈ 180 MB | ≈ 240 MB |
| 1e9 (1B)   | ≈ 1.2 GB | ≈ 1.8 GB | ≈ 2.4 GB |
| 1e10 (10B) | ≈ 12 GB  | ≈ 18 GB  | ≈ 24 GB  |

## Notes

- Counting filters use 4-bit counters; they saturate at 15 (a documented limit).
  Only `Remove` elements you actually `Add`ed.
- The default hasher is FNV-1a-128 with a splitmix64 finalizer for uniform
  distribution; it is deterministic, so persisted filters reload identically.
- Windows is supported via build-tagged mmap code (CI cross-compiles `GOOS=windows`).

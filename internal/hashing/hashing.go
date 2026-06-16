// Package hashing provides the double-hashing index generator used by the
// bloom filters, plus the default FNV-128a hasher.
package hashing

import "math/bits"

const (
	fnv128OffsetLow  = 0x62b821756295c58d
	fnv128OffsetHigh = 0x6c62272e07bb0142
	fnv128PrimeLow   = 0x13b
	fnv128PrimeShift = 24
)

// Hasher produces two 64-bit hashes of a key for Kirsch-Mitzenmacher double
// hashing. It must be deterministic across processes so persisted filters
// reload correctly.
type Hasher interface {
	Hash128(key []byte) (h1, h2 uint64)
	ID() uint8 // stable identifier recorded in serialized headers
}

// FNV128a is the default hasher: the standard library's FNV-1a 128-bit hash,
// split big-endian into two uint64 halves. Deterministic and dependency-free.
type FNV128a struct{}

// ID returns 0, the reserved identifier for the default hasher.
func (FNV128a) ID() uint8 { return 0 }

// Hash128 returns two well-mixed 64-bit values derived from FNV-1a-128(key).
// FNV-1a alone has weak bit diffusion (its low bits especially), which makes
// double-hashing indices cluster; running each half through a splitmix64
// finalizer gives near-independent, uniform values. It stays deterministic and
// dependency-free, so persisted filters reload identically.
func (FNV128a) Hash128(key []byte) (uint64, uint64) {
	hi, lo := uint64(fnv128OffsetHigh), uint64(fnv128OffsetLow)
	for _, c := range key {
		lo ^= uint64(c)
		mulHi, mulLo := bits.Mul64(fnv128PrimeLow, lo)
		mulHi += lo<<fnv128PrimeShift + fnv128PrimeLow*hi
		hi, lo = mulHi, mulLo
	}
	return mix64(hi), mix64(lo)
}

// mix64 is the splitmix64 finalizer: a strong-avalanche bijection over uint64.
func mix64(z uint64) uint64 {
	z ^= z >> 30
	z *= 0xbf58476d1ce4e5b9
	z ^= z >> 27
	z *= 0x94d049bb133111eb
	z ^= z >> 31
	return z
}

// Index returns the i-th double-hashing index into a table of m cells:
// (h1 + i*h2) mod m. uint64 overflow wraps, which is fine modulo m.
func Index(h1, h2, i, m uint64) uint64 {
	return (h1 + i*h2) % m
}

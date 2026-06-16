package bloomfilter

import (
	"io"
	"math"
	"math/bits"
	"unsafe"

	"github.com/huxint/bloomfilter/internal/hashing"
	"github.com/huxint/bloomfilter/internal/storage"
)

// BloomFilter is a classic Bloom filter: 1 bit per cell, no deletion.
// It is not safe for concurrent use.
type BloomFilter struct {
	core
}

// New creates an in-memory BloomFilter sized for n expected elements at target
// false-positive rate p. It returns an error if n==0 or p is not in (0,1).
func New(n uint64, p float64) (*BloomFilter, error) {
	if err := validate(n, p); err != nil {
		return nil, err
	}
	m, k := optimalParams(n, p)
	f := &BloomFilter{}
	f.m, f.k = m, k
	f.hasher = hashing.FNV128a{}
	f.hashID = f.hasher.ID()
	f.kind = KindBloom
	f.cellBits = 1
	f.store = storage.NewMem(int(m / 8)) // m is a multiple of 64 → m/8 is integral
	return f, nil
}

// Add inserts key. Adding the same key twice still increments AddedCount.
func (f *BloomFilter) Add(key []byte) {
	if f.store.ReadOnly() {
		panic("bloomfilter: Add on a read-only filter")
	}
	h1, h2 := f.hasher.Hash128(key)
	b := f.store.Bytes()
	for i := uint64(0); i < f.k; i++ {
		idx := hashing.Index(h1, h2, i, f.m)
		b[idx>>3] |= 1 << (idx & 7)
	}
	f.n++
}

// MightContain reports whether key may be present. False means definitely
// absent; true means probably present (subject to the false-positive rate).
func (f *BloomFilter) MightContain(key []byte) bool {
	h1, h2 := f.hasher.Hash128(key)
	b := f.store.Bytes()
	for i := uint64(0); i < f.k; i++ {
		idx := hashing.Index(h1, h2, i, f.m)
		if b[idx>>3]&(1<<(idx&7)) == 0 {
			return false
		}
	}
	return true
}

// AddString is a zero-copy convenience wrapper around Add.
func (f *BloomFilter) AddString(s string) { f.Add(stringToBytes(s)) }

// MightContainString is a zero-copy convenience wrapper around MightContain.
func (f *BloomFilter) MightContainString(s string) bool {
	return f.MightContain(stringToBytes(s))
}

// stringToBytes returns the bytes of s without copying. The result must not be
// mutated; the filters only read it.
func stringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// countSetBits returns the number of 1 bits in the filter.
func (f *BloomFilter) countSetBits() uint64 {
	var c uint64
	for _, b := range f.store.Bytes() {
		c += uint64(bits.OnesCount8(b))
	}
	return c
}

// EstimateCardinality estimates the number of distinct elements added, using
// the fill ratio: n* = -(m/k) * ln(1 - X/m), where X is the set-bit count.
func (f *BloomFilter) EstimateCardinality() uint64 {
	x := f.countSetBits()
	frac := float64(x) / float64(f.m)
	if frac >= 1 {
		return f.n // saturated; fall back to the raw counter
	}
	est := -float64(f.m) / float64(f.k) * math.Log(1-frac)
	return uint64(math.Round(est))
}

// EstimateFalsePositiveRate estimates the current false-positive probability
// from the fill ratio: (X/m)^k.
func (f *BloomFilter) EstimateFalsePositiveRate() float64 {
	x := f.countSetBits()
	return math.Pow(float64(x)/float64(f.m), float64(f.k))
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (f *BloomFilter) MarshalBinary() ([]byte, error) { return encode(&f.core), nil }

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (f *BloomFilter) UnmarshalBinary(data []byte) error {
	return decodeInto(&f.core, data, KindBloom)
}

// WriteTo implements io.WriterTo (streaming, suitable for large files).
func (f *BloomFilter) WriteTo(w io.Writer) (int64, error) { return writeTo(&f.core, w) }

// ReadFrom implements io.ReaderFrom.
func (f *BloomFilter) ReadFrom(r io.Reader) (int64, error) {
	return readFrom(&f.core, r, KindBloom)
}

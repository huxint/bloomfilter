package bloomfilter

import (
	"io"

	"github.com/huxint/bloomfilter/internal/hashing"
	"github.com/huxint/bloomfilter/internal/storage"
)

// CountingFilter is a counting Bloom filter: 4 bits per cell, supports Remove.
// Counters saturate at 15 and at 0. It is not safe for concurrent use.
//
// Only Remove elements that were actually Added; removing an element that was
// never added may corrupt others. Counters that saturate at 15 can no longer
// be safely decremented — a documented limitation.
type CountingFilter struct {
	core
}

// NewCounting creates an in-memory CountingFilter sized for n expected elements
// at target false-positive rate p.
func NewCounting(n uint64, p float64) (*CountingFilter, error) {
	if err := validate(n, p); err != nil {
		return nil, err
	}
	m, k := optimalParams(n, p)
	f := &CountingFilter{}
	f.m, f.k = m, k
	f.hasher = hashing.FNV128a{}
	f.hashID = f.hasher.ID()
	f.kind = KindCounting
	f.cellBits = 4
	f.store = storage.NewMem(int(m / 2)) // 4 bits/cell, m multiple of 64 → m/2 integral
	return f, nil
}

// counter at cell idx lives in byte idx/2, nibble (idx%2)*4.
func (f *CountingFilter) get(idx uint64) uint8 {
	b := f.store.Bytes()
	return (b[idx>>1] >> ((idx & 1) * 4)) & 0xF
}

func (f *CountingFilter) incr(idx uint64) {
	b := f.store.Bytes()
	shift := (idx & 1) * 4
	v := (b[idx>>1] >> shift) & 0xF
	if v < 15 {
		v++
	}
	b[idx>>1] = (b[idx>>1] &^ (0xF << shift)) | (v << shift)
}

func (f *CountingFilter) decr(idx uint64) {
	b := f.store.Bytes()
	shift := (idx & 1) * 4
	v := (b[idx>>1] >> shift) & 0xF
	if v == 15 {
		return // saturated: cannot safely decrement
	}
	if v > 0 {
		v--
	}
	b[idx>>1] = (b[idx>>1] &^ (0xF << shift)) | (v << shift)
}

// Add inserts key, incrementing its k counters (saturating at 15).
func (f *CountingFilter) Add(key []byte) {
	if f.store.ReadOnly() {
		panic("bloomfilter: Add on a read-only filter")
	}
	h1, h2 := f.hasher.Hash128(key)
	for i := uint64(0); i < f.k; i++ {
		f.incr(hashing.Index(h1, h2, i, f.m))
	}
	f.n++
}

// Remove deletes key, decrementing its k counters (saturating at 0). Only call
// it for elements that were actually Added.
func (f *CountingFilter) Remove(key []byte) {
	if f.store.ReadOnly() {
		panic("bloomfilter: Remove on a read-only filter")
	}
	h1, h2 := f.hasher.Hash128(key)
	for i := uint64(0); i < f.k; i++ {
		f.decr(hashing.Index(h1, h2, i, f.m))
	}
	if f.n > 0 {
		f.n--
	}
}

// MightContain reports whether key may be present (all k counters > 0).
func (f *CountingFilter) MightContain(key []byte) bool {
	h1, h2 := f.hasher.Hash128(key)
	for i := uint64(0); i < f.k; i++ {
		if f.get(hashing.Index(h1, h2, i, f.m)) == 0 {
			return false
		}
	}
	return true
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (f *CountingFilter) MarshalBinary() ([]byte, error) { return encode(&f.core), nil }

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (f *CountingFilter) UnmarshalBinary(data []byte) error {
	return decodeInto(&f.core, data, KindCounting)
}

// WriteTo implements io.WriterTo.
func (f *CountingFilter) WriteTo(w io.Writer) (int64, error) { return writeTo(&f.core, w) }

// ReadFrom implements io.ReaderFrom.
func (f *CountingFilter) ReadFrom(r io.Reader) (int64, error) {
	return readFrom(&f.core, r, KindCounting)
}

package bloomfilter

import (
	"encoding"
	"errors"
	"io"
	"math"

	"github.com/huxint/bloomfilter/internal/hashing"
	"github.com/huxint/bloomfilter/internal/storage"
)

// Kind identifies the filter variant in the serialized header.
type Kind uint8

const (
	KindBloom    Kind = 1
	KindCounting Kind = 2
)

// Filter is the common interface implemented by BloomFilter and CountingFilter.
type Filter interface {
	Add(key []byte)
	MightContain(key []byte) bool
	AddedCount() uint64    // number of Add calls (decreases on Remove for counting)
	Params() (m, k uint64) // number of cells, number of hash functions
	encoding.BinaryMarshaler
	io.WriterTo
}

// MmapFilter is a Filter backed by a memory map; callers must Close it.
type MmapFilter interface {
	Filter
	Sync() error
	Close() error
}

// core holds the fields and behavior shared by both filter types.
type core struct {
	m, k, n  uint64
	hasher   hashing.Hasher
	hashID   uint8
	kind     Kind
	cellBits uint8
	store    storage.Region
}

// AddedCount reports the number of elements added (minus removed, for counting).
func (c *core) AddedCount() uint64 { return c.n }

// Params returns the number of cells (m) and hash functions (k).
func (c *core) Params() (uint64, uint64) { return c.m, c.k }

func validate(n uint64, p float64) error {
	if n == 0 {
		return errors.New("bloomfilter: n must be > 0")
	}
	if math.IsNaN(p) || p <= 0 || p >= 1 {
		return errors.New("bloomfilter: p must be in (0,1)")
	}
	return nil
}

// optimalParams derives m (rounded up to a multiple of 64) and k from the
// expected element count n and target false-positive rate p.
func optimalParams(n uint64, p float64) (m, k uint64, err error) {
	mf := -float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)
	maxUint64Float := math.Ldexp(1, 64)
	if math.IsNaN(mf) || math.IsInf(mf, 0) || mf <= 0 || mf >= maxUint64Float {
		return 0, 0, ErrTooLarge
	}
	m = uint64(math.Ceil(mf))
	if rem := m % 64; rem != 0 {
		if m > math.MaxUint64-(64-rem) {
			return 0, 0, ErrTooLarge
		}
		m += 64 - rem
	}
	if m == 0 {
		m = 64
	}
	kf := float64(m) / float64(n) * math.Ln2
	if math.IsNaN(kf) || math.IsInf(kf, 0) || kf >= maxUint64Float {
		return 0, 0, ErrTooLarge
	}
	k = uint64(math.Round(kf))
	if k < 1 {
		k = 1
	}
	return m, k, nil
}

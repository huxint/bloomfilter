package bloomfilter

import (
	"os"

	"github.com/huxint/bloomfilter/internal/storage"
)

func cellBitsOf(kind Kind) (uint8, bool) {
	switch kind {
	case KindBloom:
		return 1, true
	case KindCounting:
		return 4, true
	default:
		return 0, false
	}
}

// newOfKind constructs an empty concrete filter for the given kind and points
// its core at store, using the supplied header fields.
func newOfKind(h header, store storage.Region) (MmapFilter, error) {
	switch h.kind {
	case KindBloom:
		f := &BloomFilter{}
		setCore(&f.core, h, store)
		return f, nil
	case KindCounting:
		f := &CountingFilter{}
		setCore(&f.core, h, store)
		return f, nil
	default:
		return nil, ErrCorrupt
	}
}

// CreateMmap creates a new file-backed filter sized for n elements at rate p.
// Use it to build filters too large to hold in RAM. The caller must Close it.
func CreateMmap(path string, kind Kind, n uint64, p float64) (MmapFilter, error) {
	if err := validate(n, p); err != nil {
		return nil, err
	}
	cellBits, ok := cellBitsOf(kind)
	if !ok {
		return nil, ErrUnknownKind
	}
	m, k := optimalParams(n, p)
	dataLen := expectedDataLen(m, cellBits)
	size := headerSize + int(dataLen)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	if err := f.Truncate(int64(size)); err != nil {
		f.Close()
		return nil, err
	}
	h := header{kind: kind, hashID: 0, cellBits: cellBits, m: m, k: k, n: 0, dataLen: dataLen}
	if _, err := f.WriteAt(h.marshal(), 0); err != nil {
		f.Close()
		return nil, err
	}
	region, err := storage.MapFile(f, size, headerSize, false)
	if err != nil {
		f.Close()
		return nil, err
	}
	return newOfKind(h, region)
}

// OpenMmap maps an existing filter file. With readOnly=true the result is for
// queries only; calling Add/Remove on it panics. The caller must Close it.
func OpenMmap(path string, readOnly bool) (MmapFilter, error) {
	flag := os.O_RDWR
	if readOnly {
		flag = os.O_RDONLY
	}
	f, err := os.OpenFile(path, flag, 0)
	if err != nil {
		return nil, err
	}
	var hbuf [headerSize]byte
	if _, err := f.ReadAt(hbuf[:], 0); err != nil {
		f.Close()
		return nil, ErrCorrupt
	}
	h, err := parseHeader(hbuf[:])
	if err != nil {
		f.Close()
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	size := headerSize + int(h.dataLen)
	if fi.Size() < int64(size) {
		f.Close()
		return nil, ErrCorrupt
	}
	region, err := storage.MapFile(f, size, headerSize, readOnly)
	if err != nil {
		f.Close()
		return nil, err
	}
	mf, err := newOfKind(h, region)
	if err != nil {
		region.Close()
		return nil, err
	}
	return mf, nil
}

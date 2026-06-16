// Package storage backs a filter's cells with either an in-memory byte slice
// or an mmap'd file. Bytes() always returns the cell-data region (the header
// is excluded). Header() exposes a writable header area for mmap backends and
// is nil for in-memory backends.
package storage

// Region is a mutable byte region backing a filter's cells.
type Region interface {
	Bytes() []byte  // the cell-data bytes (header excluded)
	Header() []byte // writable header area (mmap); nil for in-memory
	ReadOnly() bool // true for read-only mmap mappings
	Sync() error    // flush to disk (mmap: msync; mem: no-op)
	Close() error   // release resources (mmap: munmap+close; mem: no-op)
}

type memStore struct {
	data []byte
}

// NewMem returns a zeroed in-memory region of size bytes.
func NewMem(size int) Region { return &memStore{data: make([]byte, size)} }

// WrapMem wraps an existing slice as a region without copying.
func WrapMem(b []byte) Region { return &memStore{data: b} }

func (s *memStore) Bytes() []byte  { return s.data }
func (s *memStore) Header() []byte { return nil }
func (s *memStore) ReadOnly() bool { return false }
func (s *memStore) Sync() error    { return nil }
func (s *memStore) Close() error   { return nil }

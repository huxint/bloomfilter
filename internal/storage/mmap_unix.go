//go:build unix

package storage

import (
	"os"

	"golang.org/x/sys/unix"
)

type mmapRegion struct {
	file     *os.File
	full     []byte // entire mapped file (header + data)
	off      int    // data offset (== headerSize)
	readOnly bool
}

func (r *mmapRegion) Bytes() []byte  { return r.full[r.off:] }
func (r *mmapRegion) Header() []byte { return r.full[:r.off] }
func (r *mmapRegion) ReadOnly() bool { return r.readOnly }

func (r *mmapRegion) Sync() error {
	if r.readOnly {
		return nil
	}
	return unix.Msync(r.full, unix.MS_SYNC)
}

func (r *mmapRegion) Close() error {
	err := unix.Munmap(r.full)
	if cerr := r.file.Close(); err == nil {
		err = cerr
	}
	return err
}

// MapFile maps the whole file [0,size) and exposes cell bytes from off.
// The file must already be at least size bytes. The returned Region takes
// ownership of f and closes it on Close.
func MapFile(f *os.File, size, off int, readOnly bool) (Region, error) {
	prot := unix.PROT_READ
	if !readOnly {
		prot |= unix.PROT_WRITE
	}
	data, err := unix.Mmap(int(f.Fd()), 0, size, prot, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	return &mmapRegion{file: f, full: data, off: off, readOnly: readOnly}, nil
}

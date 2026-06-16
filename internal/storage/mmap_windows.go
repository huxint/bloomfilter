//go:build windows

package storage

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type mmapRegion struct {
	file     *os.File
	mapping  windows.Handle
	addr     uintptr
	full     []byte
	off      int
	readOnly bool
}

func (r *mmapRegion) Bytes() []byte  { return r.full[r.off:] }
func (r *mmapRegion) Header() []byte { return r.full[:r.off] }
func (r *mmapRegion) ReadOnly() bool { return r.readOnly }

func (r *mmapRegion) Sync() error {
	if r.readOnly {
		return nil
	}
	if err := windows.FlushViewOfFile(r.addr, uintptr(len(r.full))); err != nil {
		return err
	}
	return windows.FlushFileBuffers(windows.Handle(r.file.Fd()))
}

func (r *mmapRegion) Close() error {
	err := windows.UnmapViewOfFile(r.addr)
	if cerr := windows.CloseHandle(r.mapping); err == nil {
		err = cerr
	}
	if cerr := r.file.Close(); err == nil {
		err = cerr
	}
	return err
}

// MapFile maps the whole file [0,size) and exposes cell bytes from off.
func MapFile(f *os.File, size, off int, readOnly bool) (Region, error) {
	var pageProt, access uint32
	if readOnly {
		pageProt = windows.PAGE_READONLY
		access = windows.FILE_MAP_READ
	} else {
		pageProt = windows.PAGE_READWRITE
		access = windows.FILE_MAP_WRITE
	}
	sizeHi := uint32(uint64(size) >> 32)
	sizeLo := uint32(uint64(size) & 0xffffffff)
	h, err := windows.CreateFileMapping(windows.Handle(f.Fd()), nil, pageProt, sizeHi, sizeLo, nil)
	if err != nil {
		return nil, err
	}
	addr, err := windows.MapViewOfFile(h, access, 0, 0, uintptr(size))
	if err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	full := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	return &mmapRegion{file: f, mapping: h, addr: addr, full: full, off: off, readOnly: readOnly}, nil
}

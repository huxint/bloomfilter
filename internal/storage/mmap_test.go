//go:build unix

package storage

import (
	"os"
	"testing"
)

func TestMapFileReadWrite(t *testing.T) {
	path := t.TempDir() + "/m.bin"
	const size, off = 4096 + 16, 4096
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(int64(size)); err != nil {
		t.Fatal(err)
	}
	r, err := MapFile(f, size, off, false)
	if err != nil {
		t.Fatalf("MapFile: %v", err)
	}
	if r.ReadOnly() {
		t.Fatal("rw mapping must not be read-only")
	}
	if len(r.Bytes()) != size-off {
		t.Fatalf("Bytes len = %d, want %d", len(r.Bytes()), size-off)
	}
	if len(r.Header()) != off {
		t.Fatalf("Header len = %d, want %d", len(r.Header()), off)
	}
	r.Bytes()[0] = 0x42
	r.Header()[0] = 0x7E
	if err := r.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen read-only and confirm the byte persisted.
	f2, _ := os.OpenFile(path, os.O_RDONLY, 0)
	r2, err := MapFile(f2, size, off, true)
	if err != nil {
		t.Fatalf("MapFile ro: %v", err)
	}
	if !r2.ReadOnly() {
		t.Fatal("ro mapping must report read-only")
	}
	if r2.Bytes()[0] != 0x42 || r2.Header()[0] != 0x7E {
		t.Fatal("data did not persist across remap")
	}
	if err := r2.Sync(); err != nil { // ro Sync is a no-op
		t.Fatalf("ro Sync: %v", err)
	}
	_ = r2.Close()
}

package storage

import "testing"

func TestMemStore(t *testing.T) {
	s := NewMem(8)
	if len(s.Bytes()) != 8 {
		t.Fatalf("want 8 bytes, got %d", len(s.Bytes()))
	}
	if s.ReadOnly() {
		t.Fatal("mem store must not be read-only")
	}
	if s.Header() != nil {
		t.Fatal("mem store has no header region")
	}
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Bytes are mutable and start zeroed.
	s.Bytes()[0] = 0xAB
	if s.Bytes()[0] != 0xAB {
		t.Fatal("Bytes must be mutable and stable")
	}
}

func TestWrapMem(t *testing.T) {
	b := []byte{1, 2, 3}
	s := WrapMem(b)
	if &s.Bytes()[0] != &b[0] {
		t.Fatal("WrapMem must wrap the slice without copying")
	}
}

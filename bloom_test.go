package bloomfilter

import (
	"errors"
	"strconv"
	"testing"
)

func TestBloomNewValidation(t *testing.T) {
	if _, err := New(0, 0.01); err == nil {
		t.Fatal("New(0,...) must error")
	}
	f, err := New(1000, 0.01)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m, k := f.Params()
	if m == 0 || k == 0 {
		t.Fatalf("bad params m=%d k=%d", m, k)
	}
}

func TestBloomNewRejectsTooLarge(t *testing.T) {
	if _, err := New(^uint64(0), 0.01); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("New huge filter must be ErrTooLarge, got %v", err)
	}
}

func TestBloomNoFalseNegatives(t *testing.T) {
	f, _ := New(10_000, 0.01)
	for i := 0; i < 10_000; i++ {
		f.Add([]byte(strconv.Itoa(i)))
	}
	for i := 0; i < 10_000; i++ {
		if !f.MightContain([]byte(strconv.Itoa(i))) {
			t.Fatalf("false negative for %d (must never happen)", i)
		}
	}
	if f.AddedCount() != 10_000 {
		t.Fatalf("AddedCount: want 10000, got %d", f.AddedCount())
	}
}

func TestBloomStringHelpers(t *testing.T) {
	f, _ := New(100, 0.01)
	f.AddString("alice")
	if !f.MightContainString("alice") {
		t.Fatal("AddString/MightContainString mismatch")
	}
}

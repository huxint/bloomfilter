package bloomfilter

import (
	"strconv"
	"testing"
)

func TestCountingAddRemove(t *testing.T) {
	f, err := NewCounting(1000, 0.01)
	if err != nil {
		t.Fatalf("NewCounting: %v", err)
	}
	f.Add([]byte("alice"))
	if !f.MightContain([]byte("alice")) {
		t.Fatal("alice should be present after Add")
	}
	f.Remove([]byte("alice"))
	if f.MightContain([]byte("alice")) {
		t.Fatal("alice should be absent after Remove (no collision expected)")
	}
}

func TestCountingNoFalseNegatives(t *testing.T) {
	const n = 10_000
	f, _ := NewCounting(n, 0.01)
	for i := 0; i < n; i++ {
		f.Add([]byte(strconv.Itoa(i)))
	}
	for i := 0; i < n; i++ {
		if !f.MightContain([]byte(strconv.Itoa(i))) {
			t.Fatalf("false negative for %d", i)
		}
	}
}

func TestCountingDoubleAddSingleRemove(t *testing.T) {
	f, _ := NewCounting(1000, 0.01)
	f.Add([]byte("x"))
	f.Add([]byte("x"))
	f.Remove([]byte("x"))
	if !f.MightContain([]byte("x")) {
		t.Fatal("x added twice, removed once → still present")
	}
}

func TestCountingSaturation(t *testing.T) {
	f, _ := NewCounting(1000, 0.01)
	// Add the same key 20 times; counters saturate at 15.
	for i := 0; i < 20; i++ {
		f.Add([]byte("y"))
	}
	// 15 removes should NOT clear it (saturated counters can't track beyond 15);
	// it must still report present, demonstrating the documented limitation.
	for i := 0; i < 15; i++ {
		f.Remove([]byte("y"))
	}
	if !f.MightContain([]byte("y")) {
		t.Fatal("saturated counters must not underflow to absent after 15 removes")
	}
}

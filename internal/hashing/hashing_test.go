package hashing

import "testing"

// Pin a known deterministic output so persisted filters stay reloadable across
// builds/processes. (FNV-1a-128 of the empty input, run through splitmix64.)
func TestFNV128aEmptyStableValue(t *testing.T) {
	h1, h2 := FNV128a{}.Hash128(nil)
	if h1 != 0x292417dcc0d778ab || h2 != 0xd2ece9d449824020 {
		t.Fatalf("hash output changed (breaks persisted filters): got h1=%#x h2=%#x", h1, h2)
	}
}

func TestFNV128aDeterministic(t *testing.T) {
	a1, a2 := FNV128a{}.Hash128([]byte("alice"))
	b1, b2 := FNV128a{}.Hash128([]byte("alice"))
	if a1 != b1 || a2 != b2 {
		t.Fatal("same input must hash identically")
	}
	c1, c2 := FNV128a{}.Hash128([]byte("bob"))
	if a1 == c1 && a2 == c2 {
		t.Fatal("different inputs should (almost surely) differ")
	}
}

func TestFNV128aID(t *testing.T) {
	if (FNV128a{}).ID() != 0 {
		t.Fatal("default hasher ID must be 0")
	}
}

func TestIndex(t *testing.T) {
	// index_i = (h1 + i*h2) % m
	if got := Index(10, 3, 0, 7); got != 10%7 {
		t.Fatalf("i=0: got %d", got)
	}
	if got := Index(10, 3, 2, 7); got != (10+2*3)%7 {
		t.Fatalf("i=2: got %d", got)
	}
}

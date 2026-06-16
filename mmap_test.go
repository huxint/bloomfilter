package bloomfilter

import (
	"errors"
	"os"
	"strconv"
	"testing"
)

func TestCreateMmapBloomThenOpen(t *testing.T) {
	path := t.TempDir() + "/big.blmf"
	const n = 5000
	mf, err := CreateMmap(path, KindBloom, n, 0.01)
	if err != nil {
		t.Fatalf("CreateMmap: %v", err)
	}
	for i := 0; i < n; i++ {
		mf.Add([]byte(strconv.Itoa(i)))
	}
	if err := mf.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := mf.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen read-only; queries must match and AddedCount must persist.
	g, err := OpenMmap(path, true)
	if err != nil {
		t.Fatalf("OpenMmap: %v", err)
	}
	defer g.Close()
	if _, ok := g.(*BloomFilter); !ok {
		t.Fatalf("OpenMmap returned %T, want *BloomFilter", g)
	}
	if g.AddedCount() != n {
		t.Fatalf("AddedCount persisted = %d, want %d", g.AddedCount(), n)
	}
	for i := 0; i < n; i++ {
		if !g.MightContain([]byte(strconv.Itoa(i))) {
			t.Fatalf("missing %d after mmap reopen", i)
		}
	}
}

func TestOpenMmapReadOnlyAddPanics(t *testing.T) {
	path := t.TempDir() + "/ro.blmf"
	mf, _ := CreateMmap(path, KindBloom, 1000, 0.01)
	mf.Add([]byte("x"))
	mf.Close()

	g, err := OpenMmap(path, true)
	if err != nil {
		t.Fatalf("OpenMmap: %v", err)
	}
	defer g.Close()
	defer func() {
		if recover() == nil {
			t.Fatal("Add on read-only mmap must panic")
		}
	}()
	g.Add([]byte("y")) // must panic
}

func TestCreateMmapCounting(t *testing.T) {
	path := t.TempDir() + "/c.blmf"
	mf, err := CreateMmap(path, KindCounting, 2000, 0.01)
	if err != nil {
		t.Fatalf("CreateMmap counting: %v", err)
	}
	cf := mf.(*CountingFilter)
	cf.Add([]byte("a"))
	cf.Remove([]byte("a"))
	if cf.MightContain([]byte("a")) {
		t.Fatal("a should be absent after Remove")
	}
	mf.Close()
}

func TestOpenMmapBadFile(t *testing.T) {
	path := t.TempDir() + "/garbage.blmf"
	if err := os.WriteFile(path, []byte("not a filter"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenMmap(path, true); err == nil {
		t.Fatal("OpenMmap on garbage must error")
	}
}

func TestOpenMmapRejectsZeroK(t *testing.T) {
	path := t.TempDir() + "/zerok.blmf"
	data := append(header{kind: KindBloom, cellBits: 1, m: 64, k: 0, dataLen: 8}.marshal(), make([]byte, 8)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenMmap(path, true); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("OpenMmap with k==0 must be ErrCorrupt, got %v", err)
	}
}

func TestCreateMmapRejectsTooLarge(t *testing.T) {
	path := t.TempDir() + "/huge.blmf"
	if _, err := CreateMmap(path, KindBloom, ^uint64(0), 0.01); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("CreateMmap huge filter must be ErrTooLarge, got %v", err)
	}
}

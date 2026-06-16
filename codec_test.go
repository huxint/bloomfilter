package bloomfilter

import (
	"bytes"
	"errors"
	"strconv"
	"testing"
)

var (
	_ Filter = (*BloomFilter)(nil)
	_ Filter = (*CountingFilter)(nil)
)

func fillBloom(t *testing.T, n int) *BloomFilter {
	t.Helper()
	f, _ := New(uint64(n), 0.01)
	for i := 0; i < n; i++ {
		f.Add([]byte(strconv.Itoa(i)))
	}
	return f
}

func assertContainsAll(t *testing.T, f Filter, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if !f.MightContain([]byte(strconv.Itoa(i))) {
			t.Fatalf("missing %d after reload", i)
		}
	}
}

func TestBloomMarshalRoundTrip(t *testing.T) {
	const n = 5000
	f := fillBloom(t, n)
	data, err := f.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var g BloomFilter
	if err := g.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	gm, gk := g.Params()
	fm, fk := f.Params()
	if gm != fm || gk != fk || g.AddedCount() != f.AddedCount() {
		t.Fatal("params/count mismatch after round trip")
	}
	assertContainsAll(t, &g, n)
}

func TestBloomWriteReadRoundTrip(t *testing.T) {
	const n = 5000
	f := fillBloom(t, n)
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	var g BloomFilter
	if _, err := g.ReadFrom(&buf); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	assertContainsAll(t, &g, n)
}

func TestCountingMarshalRoundTrip(t *testing.T) {
	const n = 5000
	f, _ := NewCounting(n, 0.01)
	for i := 0; i < n; i++ {
		f.Add([]byte(strconv.Itoa(i)))
	}
	data, _ := f.MarshalBinary()
	var g CountingFilter
	if err := g.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	assertContainsAll(t, &g, n)
}

func TestUnmarshalWrongKind(t *testing.T) {
	f := fillBloom(t, 100)
	data, _ := f.MarshalBinary()
	var c CountingFilter
	if err := c.UnmarshalBinary(data); err == nil {
		t.Fatal("unmarshaling a bloom blob into a counting filter must error")
	}
}

func TestSaveLoad(t *testing.T) {
	const n = 3000
	f := fillBloom(t, n)
	path := t.TempDir() + "/f.blmf"
	if err := Save(f, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	g, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := g.(*BloomFilter); !ok {
		t.Fatalf("Load returned %T, want *BloomFilter", g)
	}
	assertContainsAll(t, g, n)
}

// Fuzz: corrupt/truncated input must error, never panic — and a successfully
// decoded filter must be safe to query (guards m==0 / overflow headers).
func FuzzUnmarshalBinary(f *testing.F) {
	sf, _ := New(64, 0.01)
	sf.Add([]byte("seed"))
	seed, _ := sf.MarshalBinary()
	f.Add(seed)
	f.Add([]byte("BLMF"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		var b BloomFilter
		if err := b.UnmarshalBinary(data); err == nil {
			b.MightContain([]byte("x")) // must not panic
		}
		var c CountingFilter
		if err := c.UnmarshalBinary(data); err == nil {
			c.MightContain([]byte("x"))
		}
		// ReadFrom is a separate decode path; exercise it too.
		var b2 BloomFilter
		if _, err := b2.ReadFrom(bytes.NewReader(data)); err == nil {
			b2.MightContain([]byte("x"))
		}
		var c2 CountingFilter
		if _, err := c2.ReadFrom(bytes.NewReader(data)); err == nil {
			c2.MightContain([]byte("x"))
		}
	})
}

// Regression: corrupt headers that previously slipped past validation and
// panicked at query time (or while allocating) must now be rejected.
func TestParseHeaderRejectsDegenerate(t *testing.T) {
	// m == 0 would divide-by-zero in Index ((h1+i*h2) % 0).
	zeroM := header{kind: KindBloom, cellBits: 1, m: 0, n: 0, dataLen: 0}.marshal()
	if _, err := parseHeader(zeroM); !errors.Is(err, ErrCorrupt) {
		t.Fatal("m==0 must be ErrCorrupt")
	}
	// m so large that m*cellBits overflows uint64, wrapping dataLen to 0.
	huge := header{kind: KindCounting, cellBits: 4, m: 1 << 62, n: 0, dataLen: 0}.marshal()
	if _, err := parseHeader(huge); !errors.Is(err, ErrCorrupt) {
		t.Fatal("overflowing m must be ErrCorrupt")
	}
}

// Regression: ReadFrom must not eagerly allocate an attacker-claimed dataLen.
func TestReadFromHugeDataLenDoesNotPanic(t *testing.T) {
	const m = uint64(1) << 40 // claims dataLen = m/8 = 2^37 bytes (~137 GB)
	h := header{kind: KindBloom, cellBits: 1, m: m, n: 0, dataLen: expectedDataLen(m, 1)}
	stream := h.marshal() // header only, no cell data follows
	var f BloomFilter
	if _, err := f.ReadFrom(bytes.NewReader(stream)); err == nil {
		t.Fatal("ReadFrom with a truncated huge body must error, not allocate/panic")
	}
}

package bloomfilter

import (
	"errors"
	"testing"
)

func TestHeaderRoundTrip(t *testing.T) {
	h := header{kind: KindBloom, hashID: 0, cellBits: 1, m: 64, k: 7, n: 3, dataLen: 8}
	buf := h.marshal()
	if len(buf) != headerSize {
		t.Fatalf("header must be %d bytes, got %d", headerSize, len(buf))
	}
	got, err := parseHeader(buf)
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if got != h {
		t.Fatalf("round-trip mismatch: %+v vs %+v", got, h)
	}
}

func TestParseHeaderErrors(t *testing.T) {
	good := header{kind: KindBloom, cellBits: 1, m: 64, k: 7, n: 0, dataLen: 8}.marshal()

	if _, err := parseHeader(good[:10]); !errors.Is(err, ErrCorrupt) {
		t.Fatal("short buffer must be ErrCorrupt")
	}

	badMagic := append([]byte(nil), good...)
	badMagic[0] = 'X'
	if _, err := parseHeader(badMagic); !errors.Is(err, ErrBadMagic) {
		t.Fatal("bad magic must be ErrBadMagic")
	}

	badVer := append([]byte(nil), good...)
	badVer[4] = 99
	if _, err := parseHeader(badVer); !errors.Is(err, ErrVersion) {
		t.Fatal("bad version must be ErrVersion")
	}

	// cellBits inconsistent with kind (bloom must be 1).
	badCell := append([]byte(nil), good...)
	badCell[7] = 4
	if _, err := parseHeader(badCell); !errors.Is(err, ErrCorrupt) {
		t.Fatal("inconsistent cellBits must be ErrCorrupt")
	}

	// dataLen inconsistent with m (m=64,cellBits=1 → dataLen must be 8).
	badLen := header{kind: KindBloom, cellBits: 1, m: 64, k: 7, dataLen: 999}.marshal()
	if _, err := parseHeader(badLen); !errors.Is(err, ErrCorrupt) {
		t.Fatal("inconsistent dataLen must be ErrCorrupt")
	}

	// k==0 would make MightContain return true for every key.
	zeroK := header{kind: KindBloom, cellBits: 1, m: 64, k: 0, dataLen: 8}.marshal()
	if _, err := parseHeader(zeroK); !errors.Is(err, ErrCorrupt) {
		t.Fatal("k==0 must be ErrCorrupt")
	}

	// Constructors never produce k>m; reject it as inconsistent/corrupt.
	hugeK := header{kind: KindBloom, cellBits: 1, m: 64, k: 65, dataLen: 8}.marshal()
	if _, err := parseHeader(hugeK); !errors.Is(err, ErrCorrupt) {
		t.Fatal("k>m must be ErrCorrupt")
	}

	// non-default hashID is unsupported.
	badHash := header{kind: KindBloom, hashID: 9, cellBits: 1, m: 64, k: 7, dataLen: 8}.marshal()
	if _, err := parseHeader(badHash); !errors.Is(err, ErrHasherMismatch) {
		t.Fatal("non-zero hashID must be ErrHasherMismatch")
	}
}

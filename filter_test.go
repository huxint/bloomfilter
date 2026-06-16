package bloomfilter

import (
	"errors"
	"math"
	"testing"
)

func TestValidate(t *testing.T) {
	if err := validate(0, 0.01); err == nil {
		t.Fatal("n=0 must error")
	}
	for _, p := range []float64{0, 1, -0.1, 1.5, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := validate(100, p); err == nil {
			t.Fatalf("p=%v must error", p)
		}
	}
	if err := validate(100, 0.01); err != nil {
		t.Fatalf("valid args must not error: %v", err)
	}
}

func TestOptimalParams(t *testing.T) {
	// 1,000,000 items @ 1% → ~9.585 bits/item ≈ 9_585_059 bits,
	// rounded up to a multiple of 64; k ≈ 7.
	m, k, err := optimalParams(1_000_000, 0.01)
	if err != nil {
		t.Fatalf("optimalParams: %v", err)
	}
	if m%64 != 0 {
		t.Fatalf("m must be a multiple of 64, got %d", m)
	}
	wantBits := -1e6 * math.Log(0.01) / (math.Ln2 * math.Ln2)
	if float64(m) < wantBits || float64(m) > wantBits+64 {
		t.Fatalf("m=%d out of expected range ~%.0f", m, wantBits)
	}
	if k != 7 {
		t.Fatalf("k: want 7, got %d", k)
	}
}

func TestOptimalParamsFloorsK(t *testing.T) {
	// Pathological tiny m/n still yields k>=1.
	_, k, err := optimalParams(1_000_000, 0.99)
	if err != nil {
		t.Fatalf("optimalParams: %v", err)
	}
	if k < 1 {
		t.Fatalf("k must be >= 1, got %d", k)
	}
}

func TestOptimalParamsRejectsTooLarge(t *testing.T) {
	if _, _, err := optimalParams(math.MaxUint64, 0.01); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("huge filter must be ErrTooLarge, got %v", err)
	}
}

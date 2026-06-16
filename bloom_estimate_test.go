package bloomfilter

import (
	"strconv"
	"testing"
)

// The measured false-positive rate must stay close to the configured target.
func TestBloomMeasuredFalsePositiveRate(t *testing.T) {
	const n = 200_000
	const p = 0.01
	f, _ := New(n, p)
	for i := 0; i < n; i++ {
		f.Add([]byte("key-" + strconv.Itoa(i)))
	}
	falsePos := 0
	const trials = 200_000
	for i := 0; i < trials; i++ {
		if f.MightContain([]byte("absent-" + strconv.Itoa(i))) {
			falsePos++
		}
	}
	rate := float64(falsePos) / float64(trials)
	t.Logf("measured FP rate = %.4f (target %.4f)", rate, p)
	if rate > 1.5*p {
		t.Fatalf("FP rate %.4f exceeds 1.5x target %.4f", rate, p)
	}
}

func TestBloomEstimateCardinality(t *testing.T) {
	const n = 50_000
	f, _ := New(n, 0.01)
	for i := 0; i < n; i++ {
		f.Add([]byte(strconv.Itoa(i)))
	}
	est := f.EstimateCardinality()
	// Within 5% of the true distinct count.
	if est < n*95/100 || est > n*105/100 {
		t.Fatalf("EstimateCardinality=%d, want ~%d", est, n)
	}
}

func TestBloomEstimateFPRateEmpty(t *testing.T) {
	f, _ := New(1000, 0.01)
	if r := f.EstimateFalsePositiveRate(); r != 0 {
		t.Fatalf("empty filter FP estimate must be 0, got %v", r)
	}
}

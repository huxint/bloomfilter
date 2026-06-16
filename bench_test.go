package bloomfilter

import (
	"strconv"
	"testing"
)

func BenchmarkBloomAdd(b *testing.B) {
	f, _ := New(uint64(b.N)+1, 0.001)
	keys := make([][]byte, 1024)
	for i := range keys {
		keys[i] = []byte("key-" + strconv.Itoa(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Add(keys[i&1023])
	}
}

func BenchmarkBloomMightContainHit(b *testing.B) {
	const n = 1_000_000
	f, _ := New(n, 0.001)
	for i := 0; i < n; i++ {
		f.Add([]byte(strconv.Itoa(i)))
	}
	keys := make([][]byte, 1024)
	for i := range keys {
		keys[i] = []byte(strconv.Itoa(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !f.MightContain(keys[i&1023]) {
			b.Fatal("unexpected miss")
		}
	}
}

func BenchmarkBloomMightContainMiss(b *testing.B) {
	const n = 1_000_000
	f, _ := New(n, 0.001)
	for i := 0; i < n; i++ {
		f.Add([]byte(strconv.Itoa(i)))
	}
	keys := make([][]byte, 1024)
	for i := range keys {
		keys[i] = []byte("absent-" + strconv.Itoa(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.MightContain(keys[i&1023])
	}
}

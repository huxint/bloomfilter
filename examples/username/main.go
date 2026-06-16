// Command username demonstrates the negative-cache pattern for username
// availability: a Bloom filter answers "definitely free" without a DB hit, and
// a CountingFilter handles username release.
package main

import (
	"fmt"
	"time"

	"github.com/huxint/bloomfilter"
)

// authoritativeDB is the source of truth (a real system would query a database).
var authoritativeDB = map[string]bool{"alice": true, "bob": true, "carol": true}

func main() {
	f, err := bloomfilter.New(1_000_000, 0.001)
	if err != nil {
		panic(err)
	}
	for name := range authoritativeDB {
		f.AddString(name)
	}

	check := func(name string) {
		start := time.Now()
		if !f.MightContainString(name) {
			fmt.Printf("%-8s -> AVAILABLE (fast path, no DB hit) [%s]\n", name, time.Since(start))
			return
		}
		// Possible hit: confirm against the authoritative store.
		taken := authoritativeDB[name]
		fmt.Printf("%-8s -> filter says maybe; DB confirms taken=%v [%s]\n", name, taken, time.Since(start))
	}

	check("alice") // taken
	check("dave")  // available (fast path)
	check("eve")   // available (fast path)

	// Username release with a counting filter.
	cf, _ := bloomfilter.NewCounting(1_000_000, 0.001)
	cf.Add([]byte("frank"))
	fmt.Printf("\nfrank present after Add:   %v\n", cf.MightContain([]byte("frank")))
	cf.Remove([]byte("frank")) // account deleted → username freed
	fmt.Printf("frank present after Remove: %v\n", cf.MightContain([]byte("frank")))
}

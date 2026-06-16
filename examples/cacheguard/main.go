// Command cacheguard demonstrates cache-penetration protection: a Bloom filter
// of existing keys lets a read-heavy service skip the DB for keys that are
// definitely absent.
package main

import (
	"fmt"

	"github.com/huxint/bloomfilter"
)

var dbQueries int

func dbLookup(key string) bool {
	dbQueries++
	// Pretend this is an expensive query against the real store.
	existing := map[string]bool{"user:1": true, "user:2": true}
	return existing[key]
}

func main() {
	f, _ := bloomfilter.New(1_000_000, 0.001)
	f.AddString("user:1")
	f.AddString("user:2")

	get := func(key string) {
		if !f.MightContainString(key) {
			fmt.Printf("%-10s -> definitely absent, DB skipped\n", key)
			return
		}
		fmt.Printf("%-10s -> maybe present, DB lookup = %v\n", key, dbLookup(key))
	}

	get("user:1")     // DB hit (real)
	get("user:99999") // skipped (flood of non-existent keys hits the filter, not the DB)
	get("user:88888") // skipped
	fmt.Printf("\ntotal DB queries: %d (without the filter it would be 3)\n", dbQueries)
}

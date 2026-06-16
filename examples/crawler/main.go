// Command crawler demonstrates a large seen-URL set that survives restarts via
// Save/Load.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/huxint/bloomfilter"
)

func main() {
	dir, _ := os.MkdirTemp("", "crawler")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "seen.blmf")

	// First run: record some crawled URLs, then persist.
	f, _ := bloomfilter.New(10_000_000, 0.001)
	for _, u := range []string{"https://a.com", "https://b.com", "https://c.com"} {
		f.AddString(u)
	}
	if err := bloomfilter.Save(f, path); err != nil {
		panic(err)
	}
	fmt.Printf("saved seen-set (%d urls)\n", f.AddedCount())

	// Second run (after restart): reload and skip already-seen URLs.
	g, err := bloomfilter.Load(path)
	if err != nil {
		panic(err)
	}
	for _, u := range []string{"https://b.com", "https://new.com"} {
		if g.MightContain([]byte(u)) {
			fmt.Printf("%-18s -> already crawled, skip\n", u)
		} else {
			fmt.Printf("%-18s -> NEW, crawl it\n", u)
			g.Add([]byte(u))
		}
	}
}

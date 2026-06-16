// Command blocklist demonstrates building a large blocklist filter once with
// CreateMmap, then loading it read-only with OpenMmap (no plaintext in memory).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/huxint/bloomfilter"
)

func main() {
	dir, _ := os.MkdirTemp("", "blocklist")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "weakpw.blmf")

	// Build phase (offline): write the blocklist into an mmap'd file.
	build, err := bloomfilter.CreateMmap(path, bloomfilter.KindBloom, 1_000_000, 0.0001)
	if err != nil {
		panic(err)
	}
	for _, pw := range []string{"123456", "password", "qwerty", "letmein"} {
		build.Add([]byte(pw))
	}
	if err := build.Close(); err != nil { // flushes to disk
		panic(err)
	}

	// Serve phase: load read-only and check candidate passwords.
	bl, err := bloomfilter.OpenMmap(path, true)
	if err != nil {
		panic(err)
	}
	defer bl.Close()

	for _, pw := range []string{"password", "Tr0ub4dour&3"} {
		if bl.MightContain([]byte(pw)) {
			fmt.Printf("%-14s -> REJECT (likely in blocklist)\n", pw)
		} else {
			fmt.Printf("%-14s -> OK (definitely not blocklisted)\n", pw)
		}
	}
}

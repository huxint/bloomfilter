// Package bloomfilter provides probabilistic set-membership filters for
// billions-scale "is this key present?" checks in milliseconds.
//
// It offers a classic BloomFilter (1 bit per cell, no deletion) and a
// CountingFilter (4 bits per cell, supports Remove). Both implement the
// Filter interface, support binary serialization, and can be backed by an
// mmap'd file for sets too large to rebuild on restart.
//
// The filters are NOT safe for concurrent use; the caller must synchronize.
package bloomfilter

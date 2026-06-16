package bloomfilter

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"

	"github.com/huxint/bloomfilter/internal/hashing"
	"github.com/huxint/bloomfilter/internal/storage"
)

const (
	headerSize    = 4096 // fixed header region; data starts here (page-aligned)
	formatVersion = 1
)

// Serialization errors returned at parse/load seams.
var (
	ErrBadMagic       = errors.New("bloomfilter: bad magic")
	ErrVersion        = errors.New("bloomfilter: unsupported format version")
	ErrCorrupt        = errors.New("bloomfilter: corrupt or inconsistent data")
	ErrHasherMismatch = errors.New("bloomfilter: unsupported hasher id")
	ErrUnknownKind    = errors.New("bloomfilter: unknown filter kind")
	ErrTooLarge       = errors.New("bloomfilter: filter too large")
)

var maxInt = int(^uint(0) >> 1)

// header is the fixed on-disk/in-buffer metadata preceding the cell data.
type header struct {
	kind     Kind
	hashID   uint8
	cellBits uint8
	m        uint64
	k        uint64
	n        uint64
	dataLen  uint64
}

// expectedDataLen returns ceil(m*cellBits/8).
func expectedDataLen(m uint64, cellBits uint8) uint64 {
	return (m*uint64(cellBits) + 7) / 8
}

func checkedDataLen(m uint64, cellBits uint8) (uint64, error) {
	if cellBits == 0 || m > (math.MaxUint64-7)/uint64(cellBits) {
		return 0, ErrTooLarge
	}
	return expectedDataLen(m, cellBits), nil
}

func checkedInt(n uint64) (int, error) {
	if n > uint64(maxInt) {
		return 0, ErrTooLarge
	}
	return int(n), nil
}

func checkedFileSize(dataLen uint64) (int, error) {
	if dataLen > uint64(maxInt-headerSize) {
		return 0, ErrTooLarge
	}
	return headerSize + int(dataLen), nil
}

// marshal renders the header into a headerSize-byte buffer (little-endian).
func (h header) marshal() []byte {
	buf := make([]byte, headerSize)
	buf[0], buf[1], buf[2], buf[3] = 'B', 'L', 'M', 'F'
	buf[4] = formatVersion
	buf[5] = byte(h.kind)
	buf[6] = h.hashID
	buf[7] = h.cellBits
	binary.LittleEndian.PutUint64(buf[8:16], h.m)
	binary.LittleEndian.PutUint64(buf[16:24], h.k)
	binary.LittleEndian.PutUint64(buf[24:32], h.n)
	binary.LittleEndian.PutUint64(buf[32:40], h.dataLen)
	return buf
}

// parseHeader validates and decodes the first headerSize bytes of b.
func parseHeader(b []byte) (header, error) {
	if len(b) < headerSize {
		return header{}, ErrCorrupt
	}
	if b[0] != 'B' || b[1] != 'L' || b[2] != 'M' || b[3] != 'F' {
		return header{}, ErrBadMagic
	}
	if b[4] != formatVersion {
		return header{}, ErrVersion
	}
	h := header{
		kind:     Kind(b[5]),
		hashID:   b[6],
		cellBits: b[7],
		m:        binary.LittleEndian.Uint64(b[8:16]),
		k:        binary.LittleEndian.Uint64(b[16:24]),
		n:        binary.LittleEndian.Uint64(b[24:32]),
		dataLen:  binary.LittleEndian.Uint64(b[32:40]),
	}
	switch h.kind {
	case KindBloom:
		if h.cellBits != 1 {
			return header{}, ErrCorrupt
		}
	case KindCounting:
		if h.cellBits != 4 {
			return header{}, ErrCorrupt
		}
	default:
		return header{}, ErrCorrupt
	}
	// m must be positive (m==0 would divide-by-zero in Index) and small enough
	// that m*cellBits cannot overflow uint64 — otherwise a bogus header could
	// claim a tiny dataLen for a huge m, leading to out-of-bounds queries.
	if h.m == 0 || h.m > (math.MaxUint64-7)/uint64(h.cellBits) {
		return header{}, ErrCorrupt
	}
	if h.k == 0 || h.k > h.m {
		return header{}, ErrCorrupt
	}
	if h.dataLen != expectedDataLen(h.m, h.cellBits) {
		return header{}, ErrCorrupt
	}
	if h.hashID != 0 {
		return header{}, ErrHasherMismatch
	}
	return h, nil
}

// setCore populates c from a parsed header and a backing region.
func setCore(c *core, h header, store storage.Region) {
	c.m, c.k, c.n = h.m, h.k, h.n
	c.kind = h.kind
	c.cellBits = h.cellBits
	c.hashID = h.hashID
	c.hasher = hashing.FNV128a{}
	c.store = store
}

// headerOf builds the current header for c.
func headerOf(c *core) header {
	return header{
		kind:     c.kind,
		hashID:   c.hashID,
		cellBits: c.cellBits,
		m:        c.m,
		k:        c.k,
		n:        c.n,
		dataLen:  uint64(len(c.store.Bytes())),
	}
}

// encode renders header + cell data into a single buffer.
func encode(c *core) []byte {
	data := c.store.Bytes()
	out := make([]byte, headerSize+len(data))
	copy(out, headerOf(c).marshal())
	copy(out[headerSize:], data)
	return out
}

// decodeInto parses data into c, requiring the given kind. It copies the cell
// data into a fresh in-memory region.
func decodeInto(c *core, data []byte, wantKind Kind) error {
	h, err := parseHeader(data)
	if err != nil {
		return err
	}
	if h.kind != wantKind {
		return ErrCorrupt
	}
	if h.dataLen > uint64(len(data)-headerSize) {
		return ErrCorrupt
	}
	size, err := checkedInt(h.dataLen)
	if err != nil {
		return err
	}
	region := make([]byte, size)
	copy(region, data[headerSize:headerSize+size])
	setCore(c, h, storage.WrapMem(region))
	return nil
}

// writeTo streams header then cell data to w.
func writeTo(c *core, w io.Writer) (int64, error) {
	var total int64
	n1, err := w.Write(headerOf(c).marshal())
	total += int64(n1)
	if err != nil {
		return total, err
	}
	n2, err := w.Write(c.store.Bytes())
	total += int64(n2)
	return total, err
}

// readFrom streams a filter from r into c, requiring the given kind.
func readFrom(c *core, r io.Reader, wantKind Kind) (int64, error) {
	var total int64
	hbuf := make([]byte, headerSize)
	n1, err := io.ReadFull(r, hbuf)
	total += int64(n1)
	if err != nil {
		return total, ErrCorrupt
	}
	h, err := parseHeader(hbuf)
	if err != nil {
		return total, err
	}
	if h.kind != wantKind {
		return total, ErrCorrupt
	}
	if _, err := checkedInt(h.dataLen); err != nil {
		return total, err
	}
	// Read incrementally through a LimitReader rather than make([]byte, dataLen):
	// a corrupt header can claim an enormous dataLen, and eagerly allocating it
	// would panic or OOM. ReadAll grows only with the bytes actually delivered.
	region, err := io.ReadAll(io.LimitReader(r, int64(h.dataLen)))
	total += int64(len(region))
	if err != nil {
		return total, ErrCorrupt
	}
	if uint64(len(region)) != h.dataLen {
		return total, ErrCorrupt // truncated stream
	}
	setCore(c, h, storage.WrapMem(region))
	return total, nil
}

// Sync flushes the filter to disk. For writable mmap-backed filters it first
// writes the current header (with up-to-date n) into the mapping; for
// read-only or in-memory filters the header write is skipped. The ReadOnly
// guard is essential: a read-only mapping's Header() bytes are PROT_READ, so
// writing to them would segfault.
func (c *core) Sync() error {
	if !c.store.ReadOnly() {
		if hdr := c.store.Header(); hdr != nil {
			copy(hdr, headerOf(c).marshal())
		}
	}
	return c.store.Sync()
}

// Close syncs then releases the backing region.
func (c *core) Close() error {
	syncErr := c.Sync()
	closeErr := c.store.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// Save writes f to path using its binary encoding.
func Save(f Filter, path string) error {
	data, err := f.MarshalBinary()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads a filter from path, dispatching on the stored kind.
func Load(path string) (Filter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	h, err := parseHeader(data)
	if err != nil {
		return nil, err
	}
	switch h.kind {
	case KindBloom:
		var f BloomFilter
		if err := f.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return &f, nil
	case KindCounting:
		var f CountingFilter
		if err := f.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return &f, nil
	default:
		return nil, ErrCorrupt
	}
}

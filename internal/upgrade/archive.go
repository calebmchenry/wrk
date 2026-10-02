package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

func verifyChecksum(manifest, archive []byte, name string) error {
	seen := map[string]bool{}
	var expected []byte
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return fail("INTEGRITY", "malformed SHA-256 manifest")
		}
		digest, err := hex.DecodeString(fields[0])
		file := strings.TrimPrefix(fields[1], "*")
		if err != nil || len(digest) != sha256.Size || file == "" || seen[file] {
			return fail("INTEGRITY", "malformed or duplicate checksum entry")
		}
		seen[file] = true
		if file == name {
			expected = digest
		}
	}
	if expected == nil {
		return fail("INTEGRITY", "manifest has no SHA-256 for %s", name)
	}
	actual := sha256.Sum256(archive)
	if !bytes.Equal(expected, actual[:]) {
		return fail("INTEGRITY", "SHA-256 mismatch for %s; installation unchanged", name)
	}
	return nil
}

// extract never uses archive paths as filesystem paths. Only one root file is allowed.
func extract(archive []byte, out io.Writer) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fail("INTEGRITY", "invalid gzip archive: %v", err)
	}
	defer gz.Close()
	limited := &io.LimitedReader{R: gz, N: maxExecutable + (1 << 20) + 1}
	tr := tar.NewReader(limited)
	h, err := tr.Next()
	if err != nil {
		return fail("INTEGRITY", "missing executable in archive: %v", err)
	}
	if h.Name != "wrk" || (h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA) || h.Linkname != "" || h.Size <= 0 || h.Size > maxExecutable {
		return fail("INTEGRITY", "archive must contain exactly one regular root executable named wrk (at most %d bytes)", maxExecutable)
	}
	for key := range h.PAXRecords {
		if strings.HasPrefix(key, "GNU.sparse") {
			return fail("INTEGRITY", "sparse archive entries are unsupported")
		}
	}
	if _, err := io.Copy(out, tr); err != nil {
		return fail("INTEGRITY", "extract executable: %v", err)
	}
	if _, err := tr.Next(); err != io.EOF {
		return fail("INTEGRITY", "unexpected or invalid additional archive entry: %v", err)
	}
	// Drain through gzip's trailer, checking its CRC and the total expanded size.
	rest, err := io.ReadAll(limited)
	if err != nil || limited.N == 0 {
		return fail("INTEGRITY", "corrupt or oversized expanded archive: %v", err)
	}
	for _, b := range rest {
		if b != 0 {
			return fail("INTEGRITY", "unexpected data after tar archive")
		}
	}
	return nil
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("command output exceeds limit")
	}
	return b.Buffer.Write(p)
}

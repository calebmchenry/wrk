package ticket

import (
	"crypto/sha256"
	"encoding/hex"
)

// Revision identifies the exact source bytes, including frontmatter and body.
// Treat it as opaque. It excludes config, filesystem metadata, and other tickets;
// identical bytes (including a restored version) have the same revision.
func Revision(source []byte) string {
	sum := sha256.Sum256(source)
	return "sha256:" + hex.EncodeToString(sum[:])
}

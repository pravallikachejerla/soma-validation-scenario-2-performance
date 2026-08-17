// Package security — SHA-256 file helper used by the seeder.
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// SHA256FileMust returns the hex-encoded SHA-256 of path. It panics on
// I/O error so it can be used at startup.
func SHA256FileMust(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

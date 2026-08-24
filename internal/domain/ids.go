package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

func NewID(prefix string, at time.Time, seed string) string {
	sum := sha256.Sum256([]byte(prefix + "|" + at.UTC().Format(time.RFC3339Nano) + "|" + seed))
	return prefix + "_" + hex.EncodeToString(sum[:8])
}

func Digest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

package domain

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)
	trimDashes   = regexp.MustCompile(`^-+|-+$`)
)

// Slugify turns a display name into a stable, readable identifier. Slugs are what
// composition YAML references, so they must stay free of generated suffixes.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = trimDashes.ReplaceAllString(s, "")
	if s == "" {
		return "capability"
	}
	if len(s) > 80 {
		s = trimDashes.ReplaceAllString(s[:80], "")
	}
	return s
}

// NewID builds a storage key: a readable slug plus a short random suffix, so ids stay
// greppable in logs while remaining unique.
func NewID(slug string) string {
	return Slugify(slug) + "-" + randomHex(6)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is unrecoverable for identity generation.
		panic("agent-composer: cannot read random bytes: " + err.Error())
	}
	return hex.EncodeToString(buf)[:n]
}

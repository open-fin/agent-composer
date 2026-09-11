package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// maxSlugRunes bounds a slug's length. Counted in runes, not bytes, so a CJK name is
// not truncated mid-character.
const maxSlugRunes = 80

// Slugify turns a display name into a stable, readable identifier. Slugs are what
// composition YAML references, so they must stay free of generated suffixes.
//
// Letters and digits of any script are preserved: an ASCII-only rule silently collapses
// non-Latin names onto each other ("智能客服助手" and "合规检查助手" both becoming a
// constant), which produces bogus collisions between unrelated capabilities.
func Slugify(name string) string {
	var (
		b        strings.Builder
		runes    int
		pendingD bool
	)
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if runes >= maxSlugRunes {
			break
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingD {
				b.WriteRune('-')
				runes++
				pendingD = false
			}
			b.WriteRune(r)
			runes++
		default:
			// Collapse any run of separators, and never start with one.
			if b.Len() > 0 {
				pendingD = true
			}
		}
	}

	slug := strings.Trim(b.String(), "-")
	if slug != "" {
		return slug
	}
	if strings.TrimSpace(name) == "" {
		return "capability"
	}
	// A name made entirely of punctuation still needs a distinct, stable identifier.
	return "capability-" + shortHash(name)
}

// NewID builds a storage key: a readable slug plus a short random suffix, so ids stay
// greppable in logs while remaining unique.
func NewID(slug string) string {
	return Slugify(slug) + "-" + randomHex(6)
}

// shortHash is a deterministic fingerprint used when a name yields no slug characters.
func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is unrecoverable for identity generation.
		panic("agent-composer: cannot read random bytes: " + err.Error())
	}
	return hex.EncodeToString(buf)[:n]
}

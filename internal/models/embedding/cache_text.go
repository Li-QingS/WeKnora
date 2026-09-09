package embedding

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// CanonicalizeCacheText removes formatting differences that do not carry
// useful meaning for retrieval embeddings. It deliberately preserves inner
// spaces, blank-line count and indentation so prose, Markdown and code are not
// aggressively rewritten.
func CanonicalizeCacheText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = norm.NFC.String(text)

	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

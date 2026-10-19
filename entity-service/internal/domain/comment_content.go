package domain

import "strings"

const (
	commentCodeOpen  = "[code]"
	commentCodeClose = "[/code]"
)

// StripCommentCodeMarkers returns content with a single wrapping
// "[code]...[/code]" pair removed. Migrated records carry these markers from
// the legacy data source's journal format; they are display noise for API
// consumers. It is a read-side presentation transform only: stored content
// and any write or mirror payload must keep the original string.
//
// Only a pair that wraps the whole value (after trimming surrounding
// whitespace) is removed. Interior markers, a lone leading or trailing
// marker, and any HTML are left untouched. Content that is not wrapped is
// returned exactly as given, whitespace included.
func StripCommentCodeMarkers(content string) string {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < len(commentCodeOpen)+len(commentCodeClose) ||
		!strings.HasPrefix(trimmed, commentCodeOpen) ||
		!strings.HasSuffix(trimmed, commentCodeClose) {
		return content
	}
	return trimmed[len(commentCodeOpen) : len(trimmed)-len(commentCodeClose)]
}

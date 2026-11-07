package memodocx

import (
	"regexp"
	"strings"
)

// metaMarker (U+2063 INVISIBLE SEPARATOR) prefixes the memo meta line; the
// Lua filter (assets/memos.lua) strips it and styles the paragraph.
const metaMarker = "\u2063"

// entry is one memo rendered as a document section.
type entry struct {
	Title    string // heading, e.g. the created time
	Subtitle string // tags · visibility (SPACE omitted) · updated
	Markdown string // memo body (already image-rewritten)
}

// buildMarkdown assembles the combined GFM document fed to pandoc. Each memo
// becomes a level-1 heading followed by a gray metadata line, its body,
// and a horizontal rule between memos. Headings inside memo bodies are
// demoted by one level so they nest under the memo heading.
func buildMarkdown(entries []entry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		if e.Title != "" {
			b.WriteString("# " + escapeInline(e.Title) + "\n\n")
		}
		if e.Subtitle != "" {
			// metaMarker tells the embedded Lua filter to apply "Memo Meta".
			b.WriteString(metaMarker + escapeInline(e.Subtitle) + "\n\n")
		}
		b.WriteString(demoteHeadings(strings.TrimSpace(e.Markdown)))
		b.WriteString("\n")
	}
	return b.String()
}

// escapeInline backslash-escapes Markdown punctuation so metadata text is
// rendered literally.
func escapeInline(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("\\`*_{}[]()<>#+-!|~", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// demoteHeadings turns ATX headings ("# x") outside fenced code into one
// level deeper (max 6) so memo headings sit below the per-memo heading.
func demoteHeadings(md string) string {
	lines := strings.Split(md, "\n")
	fence := ""
	for i, l := range lines {
		t := strings.TrimLeft(l, " ")
		if fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			fence = t[:3]
			continue
		}
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if len(l)-len(t) > 3 {
			continue
		}
		n := 0
		for n < len(t) && t[n] == '#' {
			n++
		}
		if n >= 1 && n < 6 && (len(t) == n || t[n] == ' ' || t[n] == '\t') {
			lines[i] = "#" + t
		}
	}
	return strings.Join(lines, "\n")
}

var tagOnlyLine = regexp.MustCompile(`^(#[^\s#]+)(\s+#[^\s#]+)*$`)

// stripTrailingTagLines removes blank lines and tag-only lines at the end of a
// memo body so Word/Markdown exports do not repeat tags already shown in the subtitle.
func stripTrailingTagLines(content string) string {
	lines := strings.Split(content, "\n")
	end := len(lines)
	for end > 0 {
		trimmed := strings.TrimSpace(lines[end-1])
		if trimmed == "" || tagOnlyLine.MatchString(trimmed) {
			end--
			continue
		}
		break
	}
	return strings.TrimRight(strings.Join(lines[:end], "\n"), "\n")
}

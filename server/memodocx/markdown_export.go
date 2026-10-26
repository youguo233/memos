package memodocx

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/usememos/memos/internal/httphandler"
	"github.com/usememos/memos/store"
)

// serveMemoMarkdown serves one memo as a raw Markdown file:
//
//	GET /file/memos/{uid}/md?tz=
//
// The memo body is kept verbatim; a short meta comment header and a list of
// attachment links (relative /file/attachments/... URLs) are added.
func (s *Service) serveMemoMarkdown(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	uid := r.PathValue("uid")
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &uid})
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to find memo", Cause: err}
	}
	if memo == nil {
		return &httphandler.Error{Code: http.StatusNotFound, Message: "memo not found"}
	}
	if err := s.checkRead(ctx, r, memo); err != nil {
		return err
	}
	loc := s.location(ctx, r)
	data := []byte(s.memoMarkdown(ctx, memo, loc))
	filename := "memo-" + time.Unix(memo.CreatedTs, 0).In(loc).Format("20060102-1504") + "-" + uid + ".md"
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+asciiFilename(filename)+`"; filename*=UTF-8''`+url.PathEscape(filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	return err
}

func (s *Service) memoMarkdown(ctx context.Context, memo *store.Memo, loc *time.Location) string {
	sec := section(memo, loc)
	meta := sec.Title
	if sec.Subtitle != "" {
		meta += " · " + sec.Subtitle
	}
	var b strings.Builder
	// HTML comment keeps the header out of rendered output and the body raw.
	b.WriteString("<!-- " + strings.ReplaceAll(meta, "--", "- -") + " -->\n\n")
	b.WriteString(strings.TrimRight(memo.Content, "\n"))
	b.WriteString("\n")
	atts, err := s.Store.ListAttachments(ctx, &store.FindAttachment{MemoID: &memo.ID})
	if err != nil {
		slog.Warn("markdown export: failed to list attachments", "memo", memo.UID, "error", err)
	}
	var links []string
	for _, a := range atts {
		href := "/file/attachments/" + url.PathEscape(a.UID) + "/" + url.PathEscape(a.Filename)
		if a.Reference != "" && (strings.HasPrefix(a.Reference, "http://") || strings.HasPrefix(a.Reference, "https://")) {
			href = a.Reference
		}
		if strings.Contains(memo.Content, a.UID) {
			continue // already referenced in the body
		}
		label := escapeInline(a.Filename)
		if strings.HasPrefix(a.Type, "image/") {
			links = append(links, "!["+label+"](<"+href+">)")
		} else {
			links = append(links, "- ["+label+"](<"+href+">)")
		}
	}
	if len(links) > 0 {
		b.WriteString("\n---\n\n**Attachments:**\n\n" + strings.Join(links, "\n\n") + "\n")
	}
	return b.String()
}

func asciiFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

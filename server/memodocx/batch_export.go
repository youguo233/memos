package memodocx

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/usememos/memos/internal/httphandler"
	"github.com/usememos/memos/store"
)

// maxBatchMemos caps one batch export.
const maxBatchMemos = 500

type batchRequest struct {
	// Names are memo resource names ("memos/{uid}"); each is access-checked.
	Names []string `json:"names"`
	// Tag exports the caller's own memos carrying the tag (or a nested tag below it).
	Tag string `json:"tag"`
	// Format is "md" or "docx".
	Format string `json:"format"`
	// Merge returns one combined file instead of a zip of one file per memo.
	Merge bool `json:"merge"`
}

// serveBatch handles
//
//	POST /file/memos/export?tz=   {"names":[...]|"tag":"x","format":"md|docx","merge":false}
func (s *Service) serveBatch(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return &httphandler.Error{Code: http.StatusUnsupportedMediaType, Message: "expected application/json"}
	}
	var req batchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return &httphandler.Error{Code: http.StatusBadRequest, Message: "invalid request body"}
	}
	if req.Format != "md" && req.Format != "docx" {
		return &httphandler.Error{Code: http.StatusBadRequest, Message: "format must be md or docx"}
	}
	user, err := s.currentUser(ctx, r)
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to get current user", Cause: err}
	}
	if user == nil {
		return &httphandler.Error{Code: http.StatusUnauthorized, Message: "unauthorized access"}
	}

	var memos []*store.Memo
	label := "selected"
	switch {
	case len(req.Names) > 0:
		if len(req.Names) > maxBatchMemos {
			return &httphandler.Error{Code: http.StatusBadRequest, Message: fmt.Sprintf("at most %d memos per export", maxBatchMemos)}
		}
		seen := map[string]bool{}
		for _, name := range req.Names {
			uid, ok := strings.CutPrefix(name, "memos/")
			if !ok || uid == "" || seen[uid] {
				continue
			}
			seen[uid] = true
			memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &uid})
			if err != nil {
				return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to find memo", Cause: err}
			}
			if memo == nil {
				return &httphandler.Error{Code: http.StatusNotFound, Message: "memo not found: " + name}
			}
			if err := s.checkRead(ctx, r, memo); err != nil {
				return err
			}
			memos = append(memos, memo)
		}
		slices.SortStableFunc(memos, func(a, b *store.Memo) int { return int(a.CreatedTs - b.CreatedTs) })
	case strings.TrimSpace(req.Tag) != "":
		tag := strings.TrimPrefix(strings.TrimSpace(req.Tag), "#")
		label = tag
		normal := store.Normal
		all, err := s.Store.ListMemos(ctx, &store.FindMemo{CreatorID: &user.ID, RowStatus: &normal, ExcludeComments: true, OrderByTimeAsc: true})
		if err != nil {
			return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to list memos", Cause: err}
		}
		for _, memo := range all {
			if slices.ContainsFunc(memo.Payload.GetTags(), func(t string) bool { return t == tag || strings.HasPrefix(t, tag+"/") }) {
				memos = append(memos, memo)
			}
		}
		if len(memos) > maxBatchMemos {
			return &httphandler.Error{Code: http.StatusBadRequest, Message: fmt.Sprintf("tag has %d memos; at most %d per export", len(memos), maxBatchMemos)}
		}
	default:
		return &httphandler.Error{Code: http.StatusBadRequest, Message: "names or tag is required"}
	}
	if len(memos) == 0 {
		return &httphandler.Error{Code: http.StatusNotFound, Message: "no memos to export"}
	}

	loc := s.location(ctx, r)
	stamp := time.Now().In(loc).Format("20060102-150405")
	base := "memos-" + safeName(label) + "-" + stamp

	if req.Merge {
		if req.Format == "docx" {
			data, err := s.renderDocx(ctx, "Memos · "+label, memos, loc)
			if err != nil {
				return err
			}
			return writeBytes(w, data, base+".docx")
		}
		parts := make([]string, 0, len(memos))
		for _, memo := range memos {
			parts = append(parts, s.memoMarkdown(ctx, memo, loc))
		}
		return writeFile(w, []byte(strings.Join(parts, "\n---\n\n")), "text/markdown; charset=utf-8", base+".md")
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	used := map[string]bool{}
	for _, memo := range memos {
		var data []byte
		if req.Format == "docx" {
			if data, err = s.renderDocx(ctx, "", []*store.Memo{memo}, loc); err != nil {
				return err
			}
		} else {
			data = []byte(s.memoMarkdown(ctx, memo, loc))
		}
		name := uniqueName(used, "memo-"+time.Unix(memo.CreatedTs, 0).In(loc).Format("20060102-1504")+"-"+safeName(memo.UID), "."+req.Format)
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Unix(memo.UpdatedTs, 0)})
		if err != nil {
			return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to build zip", Cause: err}
		}
		if _, err := f.Write(data); err != nil {
			return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to build zip", Cause: err}
		}
	}
	if err := zw.Close(); err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to build zip", Cause: err}
	}
	return writeFile(w, buf.Bytes(), "application/zip", base+".zip")
}

// safeName keeps letters (any script), digits, '-' and '_'; others become '_'.
func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 0x7f && r != 0xfffd:
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if len(out) > 80 {
		out = string([]rune(out)[:40])
	}
	if out == "" {
		out = "memos"
	}
	return out
}

func uniqueName(used map[string]bool, stem, ext string) string {
	name := stem + ext
	for i := 2; used[name]; i++ {
		name = stem + "-" + strconv.Itoa(i) + ext
	}
	used[name] = true
	return name
}

func writeFile(w http.ResponseWriter, data []byte, contentType, filename string) error {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+asciiFilename(filename)+`"; filename*=UTF-8''`+url.PathEscape(filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(data)
	return err
}

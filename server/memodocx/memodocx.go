// Package memodocx serves memos as Word (.docx) documents.
//
//	GET /file/memos/{uid}/docx                       one memo
//	GET /file/users/me/memos.docx?start=&end=&tz=    all of the caller's memos
//
// start and end are optional Unix seconds bounding the memo create time
// (start inclusive, end exclusive). tz is an optional IANA zone used to print
// create times; without it the caller's saved time zone preference, then
// the server's local zone, is used.
package memodocx

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/usememos/memos/core/access"
	"github.com/usememos/memos/internal/docx"
	"github.com/usememos/memos/internal/httphandler"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

// Service renders memos to .docx over plain HTTP.
type Service struct {
	Store         *store.Store
	DataDir       string // instance data dir, for local attachment storage
	authenticator *auth.Authenticator
}

// NewService creates the DOCX export service.
func NewService(st *store.Store, dataDir, secret string) *Service {
	return &Service{Store: st, DataDir: dataDir, authenticator: auth.NewAuthenticator(st, secret)}
}

// RegisterRoutes registers the export routes.
func (s *Service) RegisterRoutes(mux *http.ServeMux, fallback http.Handler) {
	mux.Handle("/file/memos/{uid}/docx", httphandler.GET(httphandler.Func(s.serveMemo), fallback))
	mux.Handle("/file/memos/{uid}/md", httphandler.GET(httphandler.Func(s.serveMemoMarkdown), fallback))
	mux.Handle("/file/users/me/memos.docx", httphandler.GET(httphandler.Func(s.serveAll), fallback))
}

func (s *Service) currentUser(ctx context.Context, r *http.Request) (*store.User, error) {
	return s.authenticator.AuthenticateToUser(ctx, r.Header.Get("Authorization"), r.Header.Get("Cookie"))
}

func (s *Service) serveMemo(w http.ResponseWriter, r *http.Request) error {
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
	return s.render(ctx, w, "", []*store.Memo{memo}, s.location(ctx, r), "memo-"+uid+".docx")
}

func (s *Service) serveAll(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	user, err := s.currentUser(ctx, r)
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to get current user", Cause: err}
	}
	if user == nil {
		return &httphandler.Error{Code: http.StatusUnauthorized, Message: "unauthorized access"}
	}
	start, err := unixParam(r, "start")
	if err != nil {
		return err
	}
	end, err := unixParam(r, "end")
	if err != nil {
		return err
	}
	normal := store.Normal
	memos, err := s.Store.ListMemos(ctx, &store.FindMemo{CreatorID: &user.ID, RowStatus: &normal, ExcludeComments: true, OrderByTimeAsc: true})
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to list memos", Cause: err}
	}
	loc := s.location(ctx, r)
	name := user.Nickname
	if name == "" {
		name = user.Username
	}
	var selected []*store.Memo
	for _, memo := range memos {
		if (start != nil && memo.CreatedTs < *start) || (end != nil && memo.CreatedTs >= *end) {
			continue
		}
		selected = append(selected, memo)
	}
	return s.render(ctx, w, "Memos · "+name, selected, loc, fmt.Sprintf("memos-%s-%s.docx", user.Username, time.Now().In(loc).Format("20060102-150405")))
}

// render converts memos with pandoc when available, otherwise with the
// built-in pure-Go generator.
func (s *Service) render(ctx context.Context, w http.ResponseWriter, title string, memos []*store.Memo, loc *time.Location, filename string) error {
	if bin := pandocPath(); bin != "" {
		entries := make([]entry, 0, len(memos))
		for _, memo := range memos {
			sec := section(memo, loc)
			entries = append(entries, entry{Title: sec.Title, Subtitle: sec.Subtitle, Markdown: s.memoImages(ctx, memo)})
		}
		md := buildMarkdown(entries)
		if len(entries) == 0 {
			md = "_No memos in the selected range._\n"
		}
		out, err := pandocConvert(ctx, bin, md, title)
		if err == nil {
			return writeBytes(w, out, filename)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("docx export: pandoc conversion failed, falling back to built-in generator", "error", err)
	} else {
		pandocMissingOnce.Do(func() {
			slog.Warn("docx export: pandoc not found in PATH (or MEMOS_PANDOC_PATH); using built-in generator with limited Markdown support")
		})
	}
	doc := &docx.Document{Title: title}
	for _, memo := range memos {
		doc.Sections = append(doc.Sections, section(memo, loc))
	}
	if len(doc.Sections) == 0 {
		doc.Sections = []docx.Section{{Markdown: "_No memos in the selected range._"}}
	}
	return write(w, doc, filename)
}

var pandocMissingOnce sync.Once

func section(memo *store.Memo, loc *time.Location) docx.Section {
	var sub []string
	if tags := memo.Payload.GetTags(); len(tags) > 0 {
		sub = append(sub, "#"+strings.Join(tags, " #"))
	}
	sub = append(sub, string(memo.Visibility))
	if memo.UpdatedTs != memo.CreatedTs {
		sub = append(sub, "updated "+time.Unix(memo.UpdatedTs, 0).In(loc).Format("2006-01-02 15:04"))
	}
	return docx.Section{
		Title:    time.Unix(memo.CreatedTs, 0).In(loc).Format("2006-01-02 15:04 MST"),
		Subtitle: strings.Join(sub, " · "),
		Markdown: memo.Content,
	}
}

func write(w http.ResponseWriter, doc *docx.Document, filename string) error {
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to render docx", Cause: err}
	}
	return writeBytes(w, buf.Bytes(), filename)
}

func writeBytes(w http.ResponseWriter, data []byte, filename string) error {
	w.Header().Set("Content-Type", docx.MediaType)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(data)
	return err
}

// location resolves the zone for printed times: the tz query parameter, then
// the caller's saved custom time zone preference, then the server's local zone.
func (s *Service) location(ctx context.Context, r *http.Request) *time.Location {
	if tz := r.URL.Query().Get("tz"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	if user, err := s.currentUser(ctx, r); err == nil && user != nil {
		key := storepb.UserSetting_GENERAL
		if setting, err := s.Store.GetUserSetting(ctx, &store.FindUserSetting{UserID: &user.ID, Key: key}); err == nil && setting != nil {
			if tz := setting.GetGeneral().GetTimezone(); tz != "" {
				if loc, err := time.LoadLocation(tz); err == nil {
					return loc
				}
			}
		}
	}
	return time.Local
}

func unixParam(r *http.Request, key string) (*int64, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, &httphandler.Error{Code: http.StatusBadRequest, Message: "invalid " + key}
	}
	return &v, nil
}

// checkRead applies the same read rules as the memo API.
func (s *Service) checkRead(ctx context.Context, r *http.Request, memo *store.Memo) error {
	allowAnonymous, err := s.Store.AllowsAnonymousAccess(ctx)
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to get instance access policy", Cause: err}
	}
	facts, err := access.ResolveMemoReadFacts(ctx, s.Store, memo)
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to resolve memo access", Cause: err}
	}
	user, err := s.currentUser(ctx, r)
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to get current user", Cause: err}
	}
	readContext, err := facts.WithViewer(ctx, s.Store, user, allowAnonymous, nil)
	if err != nil {
		return &httphandler.Error{Code: http.StatusInternalServerError, Message: "failed to resolve memo access", Cause: err}
	}
	switch access.CheckMemoReadContext(readContext).Denial {
	case access.MemoReadDenialNone:
		return nil
	case access.MemoReadDenialNotFound:
		return &httphandler.Error{Code: http.StatusNotFound, Message: "memo not found"}
	case access.MemoReadDenialUnauthenticated:
		return &httphandler.Error{Code: http.StatusUnauthorized, Message: "unauthorized access"}
	default:
		return &httphandler.Error{Code: http.StatusForbidden, Message: "forbidden access"}
	}
}

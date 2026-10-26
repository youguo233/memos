package memodocx

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

const (
	maxImageBytes      = 20 << 20
	remoteImageTimeout = 8 * time.Second
)

// imageRe matches inline Markdown images: ![alt](url "title").
var imageRe = regexp.MustCompile(`!\[([^\]]*)\]\(\s*<?([^)\s>]+)>?(\s+"[^"]*")?\s*\)`)

// attachmentRe extracts the attachment uid from a Memos file URL, absolute or
// relative (".../file/attachments/{uid}[/{filename}]").
var attachmentRe = regexp.MustCompile(`^(?:https?://[^/]+)?/file/attachments/([^/?#]+)`)

// memoImages resolves images for one memo. Every image in the Markdown is
// rewritten to a data: URI (so pandoc can run in --sandbox and never touches
// the server filesystem or network) or, when it cannot be resolved, demoted
// to a plain link. Image attachments of the memo that are not referenced
// inline are appended; other attachments are listed by name.
func (s *Service) memoImages(ctx context.Context, memo *store.Memo) string {
	atts, err := s.Store.ListAttachments(ctx, &store.FindAttachment{MemoID: &memo.ID})
	if err != nil {
		slog.Warn("docx export: failed to list attachments", "memo", memo.UID, "error", err)
	}
	byUID := map[string]*store.Attachment{}
	for _, a := range atts {
		byUID[a.UID] = a
	}
	used := map[string]bool{}
	cache := map[string]string{}

	content := rewriteImages(memo.Content, func(url string) string {
		if m := attachmentRe.FindStringSubmatch(url); m != nil {
			a := byUID[m[1]]
			if a == nil {
				// Only attachments owned by the memo's creator may be inlined,
				// so a memo cannot pull in someone else's private file.
				uid := m[1]
				found, err := s.Store.GetAttachment(ctx, &store.FindAttachment{UID: &uid})
				if err != nil || found == nil || found.CreatorID != memo.CreatorID {
					return ""
				}
				a = found
			}
			used[a.UID] = true
			if v, ok := cache[a.UID]; ok {
				return v
			}
			v := s.attachmentDataURI(ctx, a)
			cache[a.UID] = v
			return v
		}
		if strings.HasPrefix(url, "data:image/") {
			return url
		}
		if (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) && remoteImagesEnabled() {
			return fetchRemoteImage(ctx, url)
		}
		return ""
	})

	var extra, files []string
	for _, a := range atts {
		if used[a.UID] {
			continue
		}
		if strings.HasPrefix(a.Type, "image/") {
			if uri := s.attachmentDataURI(ctx, a); uri != "" {
				extra = append(extra, "!["+escapeInline(a.Filename)+"]("+uri+")")
				continue
			}
		}
		files = append(files, "- "+escapeInline(a.Filename))
	}
	if len(extra) > 0 {
		content += "\n\n" + strings.Join(extra, "\n\n")
	}
	if len(files) > 0 {
		content += "\n\n*Attachments:*\n\n" + strings.Join(files, "\n")
	}
	return content
}

// rewriteImages replaces each image URL via resolve; an empty result turns the
// image into a link (or plain alt text) so nothing unresolved reaches pandoc.
// Fenced code blocks are left untouched.
func rewriteImages(md string, resolve func(string) string) string {
	lines := strings.SplitAfter(md, "\n")
	fence := ""
	var out strings.Builder
	for _, l := range lines {
		t := strings.TrimLeft(l, " ")
		if fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			fence = t[:3]
			out.WriteString(l)
			continue
		}
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			out.WriteString(l)
			continue
		}
		out.WriteString(imageRe.ReplaceAllStringFunc(l, func(m string) string {
			sm := imageRe.FindStringSubmatch(m)
			alt, url := sm[1], sm[2]
			if v := resolve(url); v != "" {
				return "![" + alt + "](" + v + ")"
			}
			if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
				label := alt
				if label == "" {
					label = url
				}
				return "[" + label + "](" + url + ")"
			}
			if alt == "" {
				return ""
			}
			return "[image: " + alt + "]"
		}))
	}
	return out.String()
}

func (s *Service) attachmentDataURI(ctx context.Context, a *store.Attachment) string {
	if !strings.HasPrefix(a.Type, "image/") || a.Size > maxImageBytes {
		return ""
	}
	blob, err := s.attachmentBlob(ctx, a)
	if err != nil {
		slog.Warn("docx export: failed to read attachment", "attachment", a.UID, "error", err)
		return ""
	}
	return dataURI(a.Type, blob)
}

func dataURI(mime string, blob []byte) string {
	if len(blob) == 0 || len(blob) > maxImageBytes {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(blob)
}

// attachmentBlob reads attachment content from DB, local disk, or S3.
func (s *Service) attachmentBlob(ctx context.Context, a *store.Attachment) ([]byte, error) {
	switch a.StorageType {
	case storepb.AttachmentStorageType_LOCAL:
		p := filepath.FromSlash(a.Reference)
		if !filepath.IsAbs(p) {
			p = filepath.Join(s.DataDir, p)
		}
		return os.ReadFile(p)
	case storepb.AttachmentStorageType_S3:
		driver, obj, err := s.Store.ResolveAttachmentS3Driver(ctx, a)
		if err != nil {
			return nil, err
		}
		o, err := driver.GetObjectStream(ctx, obj.Key, "")
		if err != nil {
			return nil, errors.Wrap(err, "failed to stream from S3")
		}
		defer o.Body.Close()
		return io.ReadAll(io.LimitReader(o.Body, maxImageBytes+1))
	default:
		if len(a.Blob) == 0 {
			full, err := s.Store.GetAttachment(ctx, &store.FindAttachment{ID: &a.ID, GetBlob: true})
			if err != nil || full == nil {
				return nil, errors.New("attachment blob not found")
			}
			return full.Blob, nil
		}
		return a.Blob, nil
	}
}

func remoteImagesEnabled() bool {
	v := strings.ToLower(os.Getenv("MEMOS_DOCX_REMOTE_IMAGES"))
	return v == "1" || v == "true" || v == "yes"
}

// fetchRemoteImage downloads an external image (opt-in) with a short timeout.
func fetchRemoteImage(ctx context.Context, url string) string {
	ctx, cancel := context.WithTimeout(ctx, remoteImageTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("docx export: remote image skipped", "url", url, "error", err)
		return ""
	}
	defer resp.Body.Close()
	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(mime, "image/") {
		return ""
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(resp.Body, maxImageBytes+1)); err != nil {
		return ""
	}
	return dataURI(mime, buf.Bytes())
}

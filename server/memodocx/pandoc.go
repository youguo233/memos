package memodocx

import (
	"bytes"
	"context"
	_ "embed"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// defaultReferenceDoc is pandoc's reference.docx with CJK-friendly fonts
// (Calibri / Microsoft YaHei, SimSun for code) and zh-CN East Asian language.
//
//go:embed assets/reference.docx
var defaultReferenceDoc []byte

// luaFilter styles the memo meta line and separators (see assets/memos.lua).
//
//go:embed assets/memos.lua
var luaFilter []byte

const pandocTimeout = 2 * time.Minute

var (
	refOnce sync.Once
	refPath string
	luaPath string
)

// pandocPath returns the pandoc binary (MEMOS_PANDOC_PATH or $PATH), or "".
func pandocPath() string {
	if p := os.Getenv("MEMOS_PANDOC_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p, err := exec.LookPath("pandoc")
	if err != nil {
		return ""
	}
	return p
}

// referenceDoc returns MEMOS_DOCX_REFERENCE_DOC if set, else the embedded
// default. The embedded defaults (reference doc and Lua filter) are written
// once to a temp dir.
func referenceDoc() string {
	refOnce.Do(func() {
		dir, err := os.MkdirTemp("", "memos-docx-ref-")
		if err != nil {
			return
		}
		p := filepath.Join(dir, "reference.docx")
		if os.WriteFile(p, defaultReferenceDoc, 0o600) == nil {
			refPath = p
		}
		l := filepath.Join(dir, "memos.lua")
		if os.WriteFile(l, luaFilter, 0o600) == nil {
			luaPath = l
		}
	})
	if p := os.Getenv("MEMOS_DOCX_REFERENCE_DOC"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		slog.Warn("docx export: MEMOS_DOCX_REFERENCE_DOC not readable, using default", "path", p)
	}
	return refPath
}

// pandocConvert converts GFM markdown to docx with pandoc. All images must
// already be data: URIs; pandoc runs with --sandbox so it reads nothing else.
func pandocConvert(ctx context.Context, bin, markdown, title string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, pandocTimeout)
	defer cancel()
	args := []string{"-f", "gfm+hard_line_breaks", "-t", "docx", "--sandbox", "-o", "-"}
	if ref := referenceDoc(); ref != "" {
		args = append(args, "--reference-doc", ref)
	}
	if luaPath != "" {
		args = append(args, "--lua-filter", luaPath)
	}
	if title != "" {
		args = append(args, "--metadata", "title="+title)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = bytes.NewReader([]byte(markdown))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "pandoc failed: %s", stderr.String())
	}
	if stdout.Len() == 0 {
		return nil, errors.New("pandoc produced no output")
	}
	out, err := polishDocx(stdout.Bytes())
	if err != nil {
		slog.Warn("docx export: post-processing failed, using raw pandoc output", "error", err)
		return stdout.Bytes(), nil
	}
	return out, nil
}

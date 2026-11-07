package memodocx

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func TestBuildMarkdown(t *testing.T) {
	md := buildMarkdown([]entry{
		{Title: "2026-10-26 10:00 CST", Subtitle: "#work #笔记 · PRIVATE", Markdown: "# 标题\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\n# not a heading\n```"},
		{Title: "2026-10-27 09:00 CST", Subtitle: "PUBLIC", Markdown: "- [x] done"},
	})
	for _, want := range []string{
		"# 2026\\-10\\-26 10:00 CST\n\n",
		metaMarker + "\\#work \\#笔记 · PRIVATE\n\n",
		"## 标题\n", // demoted from #
		"| a | b |",
		"```go\n# not a heading\n```", // code untouched
		"\n\n---\n\n# 2026\\-10\\-27",
		"- [x] done",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestRewriteImages(t *testing.T) {
	in := "![a](/file/attachments/abc/x.png) ![b](https://ex.com/y.png) ![](/etc/passwd)\n```\n![c](/etc/passwd)\n```\n"
	out := rewriteImages(in, func(u string) string {
		if strings.Contains(u, "/file/attachments/abc") {
			return "data:image/png;base64,AA=="
		}
		return ""
	})
	want := "![a](data:image/png;base64,AA==) [b](https://ex.com/y.png) \n```\n![c](/etc/passwd)\n```\n"
	if out != want {
		t.Fatalf("got %q\nwant %q", out, want)
	}
	if m := attachmentRe.FindStringSubmatch("https://memos.example.com/file/attachments/U1/a.png"); m == nil || m[1] != "U1" {
		t.Fatalf("attachmentRe: %v", m)
	}
}

// 1x1 PNG.
const pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestPandocConvert(t *testing.T) {
	bin := pandocPath()
	if bin == "" {
		t.Skip("pandoc not installed")
	}
	md := buildMarkdown([]entry{{Title: "t", Subtitle: "#标签 · PRIVATE", Markdown: "中文\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n- [ ] todo\n- [x] done\n\n```go\nfunc main() {}\n```\n\n![img](data:image/png;base64," + pngB64 + ")"}})
	out, err := pandocConvert(context.Background(), bin, md, "Memos")
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	var doc, numbering string
	media := false
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "word/media/") {
			media = true
		}
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			doc = string(b)
		}
		if f.Name == "word/numbering.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			numbering = string(b)
		}
	}
	if !media {
		t.Error("no embedded image in word/media")
	}
	for _, want := range []string{"<w:tbl>", "中文", "todo", "func", `w:val="MemoMeta"`, `<w:tblW w:type="pct" w:w="5000"/>`} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml missing %q", want)
		}
	}
	if strings.Contains(doc, "\u2063") {
		t.Error("meta marker leaked into document")
	}
	if os.Getenv("MEMOS_DOCX_SAMPLE_OUT") != "" {
		_ = os.WriteFile(os.Getenv("MEMOS_DOCX_SAMPLE_OUT"), out, 0o644)
	}
	if !strings.Contains(numbering, "☐") {
		t.Error("task list checkbox bullet missing")
	}
	if strings.Contains(numbering, "\u2612") || strings.Contains(doc, "\u2612") {
		t.Error("ugly ☒ checkbox leaked; want ☑")
	}
	if !strings.Contains(numbering, "\u2611") {
		t.Error("checked task checkbox ☑ missing")
	}
}

func TestSafeAndUniqueName(t *testing.T) {
	if got := safeName("调试/a b"); got != "调试_a_b" {
		t.Fatalf("safeName = %q", got)
	}
	used := map[string]bool{}
	if a, b := uniqueName(used, "x", ".md"), uniqueName(used, "x", ".md"); a != "x.md" || b != "x-2.md" {
		t.Fatalf("uniqueName = %q %q", a, b)
	}
}

func TestStripTrailingTagLines(t *testing.T) {
	got := stripTrailingTagLines("现象\n处理说明。\n\n#调试日志\n")
	if got != "现象\n处理说明。" {
		t.Fatalf("got %q", got)
	}
	got = stripTrailingTagLines("正文\n\n#foo #bar\n")
	if got != "正文" {
		t.Fatalf("got %q", got)
	}
	got = stripTrailingTagLines("# 标题\n\n正文")
	if got != "# 标题\n\n正文" {
		t.Fatalf("should keep headings, got %q", got)
	}
}

func TestSectionOmitsSpaceAndTrailingTags(t *testing.T) {
	memo := &store.Memo{
		Content:    "现象\n处理说明。\n\n#调试日志\n",
		Visibility: store.SpaceAudience,
		CreatedTs:  1730800000,
		UpdatedTs:  1730800200,
		Payload:    &storepb.MemoPayload{Tags: []string{"调试日志"}},
	}
	sec := section(memo, time.UTC)
	if strings.Contains(sec.Subtitle, "SPACE") {
		t.Fatalf("subtitle should omit SPACE: %q", sec.Subtitle)
	}
	if !strings.Contains(sec.Subtitle, "#调试日志") {
		t.Fatalf("subtitle should keep tags: %q", sec.Subtitle)
	}
	if strings.Contains(sec.Markdown, "#调试日志") {
		t.Fatalf("body should drop trailing tags: %q", sec.Markdown)
	}
	if !strings.Contains(sec.Markdown, "处理说明。") {
		t.Fatalf("body should keep content: %q", sec.Markdown)
	}

	private := &store.Memo{Visibility: store.Private, CreatedTs: 1, UpdatedTs: 1}
	if sub := section(private, time.UTC).Subtitle; sub != "PRIVATE" {
		t.Fatalf("PRIVATE should still appear, got %q", sub)
	}
}

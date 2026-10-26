package memodocx

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestBuildMarkdown(t *testing.T) {
	md := buildMarkdown([]entry{
		{Title: "2026-10-26 10:00 CST", Subtitle: "#work #笔记 · PRIVATE", Markdown: "# 标题\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\n# not a heading\n```"},
		{Title: "2026-10-27 09:00 CST", Subtitle: "PUBLIC", Markdown: "- [x] done"},
	})
	for _, want := range []string{
		"# 2026\\-10\\-26 10:00 CST\n\n",
		"*\\#work \\#笔记 · PRIVATE*\n\n",
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
	md := buildMarkdown([]entry{{Title: "t", Markdown: "中文\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n- [ ] todo\n\n```go\nfunc main() {}\n```\n\n![img](data:image/png;base64," + pngB64 + ")"}})
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
	for _, want := range []string{"<w:tbl>", "中文", "todo", "func"} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml missing %q", want)
		}
	}
	if !strings.Contains(numbering, "☐") {
		t.Error("task list checkbox bullet missing")
	}
}

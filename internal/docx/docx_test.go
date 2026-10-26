package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	doc := &Document{Title: "T", Sections: []Section{{
		Title:    "2026-10-26 10:00 UTC",
		Subtitle: "#work",
		Markdown: "# 标题\n\n你好 **粗体** *斜体* `code` [link](https://example.com?a=1&b=2) #tag\n\n- a\n- b\n  1. c\n\n```go\nfmt.Println(\"<x>\")\n```\n\n> quote\n",
	}}}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
		// Every part must be well-formed XML.
		d := xml.NewDecoder(bytes.NewReader(b))
		for {
			if _, err := d.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s: %v", f.Name, err)
			}
		}
	}
	body := parts["word/document.xml"]
	for _, want := range []string{"标题", "粗体", "<w:b/>", "<w:i/>", "Consolas", "#tag", "fmt.Println(&quot;&lt;x&gt;&quot;)", `w:numId w:val="1"`, `w:numId w:val="2"`, "Heading2", "Quote", "w:hyperlink"} {
		if !strings.Contains(body, want) {
			t.Errorf("document.xml missing %q", want)
		}
	}
	if !strings.Contains(parts["word/_rels/document.xml.rels"], "https://example.com?a=1&amp;b=2") {
		t.Error("hyperlink relationship missing")
	}
}

func TestTaskCheckbox(t *testing.T) {
	doc := &Document{Title: "T", Sections: []Section{{Markdown: "- [x] done\n- [ ] todo\n"}}}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		s := string(b)
		if !strings.Contains(s, "\u2611") || !strings.Contains(s, "\u2610") || strings.Contains(s, "\u2612") {
			t.Errorf("want ☑/☐ checkboxes and no ☒")
		}
	}
}

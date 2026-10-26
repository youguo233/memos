// Package docx writes minimal, dependency-free Office Open XML (.docx)
// documents from Markdown. It maps headings, paragraphs, lists, block quotes,
// fenced and indented code (monospace), emphasis, strikethrough, inline code,
// links, tables (as tab-separated lines), and Memos #tags. It is intended for
// human-readable exports, not round-tripping.
package docx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	mast "github.com/usememos/memos/markdown/ast"
	"github.com/usememos/memos/markdown/extensions"
)

// MediaType is the media type of a .docx file.
const MediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

var md = goldmark.New(goldmark.WithExtensions(
	extension.Table, extension.Strikethrough, extension.TaskList, extensions.TagExtension,
))

// Section is one titled block of the document, e.g. a single memo.
type Section struct {
	Title    string // rendered as Heading 1; empty to omit
	Subtitle string // rendered as a small grey line under the title; optional
	Markdown string // body
}

// Document collects sections and writes them as a .docx file.
type Document struct {
	Title    string // optional document title (Title style)
	Sections []Section
}

type style struct {
	bold, italic, strike, code bool
	link                       string
}

type builder struct {
	body  strings.Builder
	rels  []string // hyperlink targets; rId = "rIdL{n}"
	src   []byte
	numID int // 1 bullet, 2 decimal
}

// Write encodes the document as .docx into w.
func (d *Document) Write(w io.Writer) error {
	b := &builder{}
	if d.Title != "" {
		b.para("Title", "", []run{{text: d.Title}})
	}
	for i, s := range d.Sections {
		if i > 0 && s.Title != "" {
			b.body.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
		}
		if s.Title != "" {
			b.para("Heading1", "", []run{{text: s.Title}})
		}
		if s.Subtitle != "" {
			b.para("Subtitle", "", []run{{text: s.Subtitle}})
		}
		b.src = []byte(s.Markdown)
		doc := md.Parser().Parse(text.NewReader(b.src))
		b.blocks(doc, 0, "")
	}
	return b.writeZip(w)
}

type run struct {
	text string
	style
	br bool
}

func (b *builder) blocks(parent gast.Node, depth int, pStyle string) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		b.block(n, depth, pStyle)
	}
}

func (b *builder) block(n gast.Node, depth int, pStyle string) {
	{
		switch v := n.(type) {
		case *gast.Heading:
			lvl := v.Level + 1 // memo headings sit below the section heading
			if lvl > 6 {
				lvl = 6
			}
			b.para(fmt.Sprintf("Heading%d", lvl), "", b.inlines(v, style{}))
		case *gast.Paragraph, *gast.TextBlock:
			b.para(pStyleOr(pStyle, "Normal"), "", b.inlines(n, style{}))
		case *gast.List:
			b.list(v, depth)
		case *gast.Blockquote:
			b.blocks(v, depth, "Quote")
		case *gast.FencedCodeBlock, *gast.CodeBlock:
			lines := n.Lines()
			var runs []run
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				runs = append(runs, run{text: strings.TrimRight(string(seg.Value(b.src)), "\r\n"), style: style{code: true}, br: i < lines.Len()-1})
			}
			if len(runs) == 0 {
				runs = []run{{text: "", style: style{code: true}}}
			}
			b.para("Code", "", runs)
		case *gast.ThematicBreak:
			b.body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="auto"/></w:pBdr></w:pPr></w:p>`)
		case *east.Table:
			for row := v.FirstChild(); row != nil; row = row.NextSibling() {
				var runs []run
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					if cell != row.FirstChild() {
						runs = append(runs, run{text: "\t"})
					}
					st := style{}
					if _, ok := row.(*east.TableHeader); ok {
						st.bold = true
					}
					runs = append(runs, b.inlines(cell, st)...)
				}
				b.para("Normal", "", runs)
			}
		case *gast.HTMLBlock:
			lines := v.Lines()
			var sb strings.Builder
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				sb.Write(seg.Value(b.src))
			}
			b.para("Normal", "", []run{{text: strings.TrimSpace(sb.String())}})
		default:
			if n.HasChildren() {
				b.blocks(n, depth, pStyle)
			}
		}
	}
}

func pStyleOr(s, d string) string {
	if s != "" {
		return s
	}
	return d
}

func (b *builder) list(l *gast.List, depth int) {
	num := 1
	if l.IsOrdered() {
		num = 2
	}
	numPr := fmt.Sprintf(`<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`, min(depth, 8), num)
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		first := true
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			switch c.(type) {
			case *gast.Paragraph, *gast.TextBlock:
				runs := b.inlines(c, style{})
				if first {
					b.para("ListParagraph", numPr, runs)
				} else {
					b.para("ListParagraph", fmt.Sprintf(`<w:ind w:left="%d"/>`, 720*(depth+1)), runs)
				}
				first = false
			case *gast.List:
				b.list(c.(*gast.List), depth+1)
			default:
				b.block(c, depth+1, "")
				first = false
			}
		}
	}
}

func (b *builder) inlines(parent gast.Node, st style) []run {
	var out []run
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch v := n.(type) {
		case *gast.Text:
			out = append(out, run{text: string(v.Segment.Value(b.src)), style: st})
			if v.HardLineBreak() || v.SoftLineBreak() {
				out = append(out, run{br: true, style: st})
			}
		case *gast.String:
			out = append(out, run{text: string(v.Value), style: st})
		case *gast.CodeSpan:
			s := st
			s.code = true
			out = append(out, run{text: nodeText(v, b.src), style: s})
		case *gast.Emphasis:
			s := st
			if v.Level >= 2 {
				s.bold = true
			} else {
				s.italic = true
			}
			out = append(out, b.inlines(v, s)...)
		case *east.Strikethrough:
			s := st
			s.strike = true
			out = append(out, b.inlines(v, s)...)
		case *gast.Link:
			s := st
			s.link = string(v.Destination)
			out = append(out, b.inlines(v, s)...)
		case *gast.AutoLink:
			s := st
			s.link = string(v.URL(b.src))
			out = append(out, run{text: string(v.Label(b.src)), style: s})
		case *gast.Image:
			s := st
			s.link = string(v.Destination)
			label := nodeText(v, b.src)
			if label == "" {
				label = string(v.Destination)
			}
			out = append(out, run{text: "[image: " + label + "]", style: s})
		case *east.TaskCheckBox:
			if v.IsChecked {
				out = append(out, run{text: "☑ ", style: st})
			} else {
				out = append(out, run{text: "☐ ", style: st})
			}
		case *gast.RawHTML:
			segs := v.Segments
			for i := 0; i < segs.Len(); i++ {
				seg := segs.At(i)
				out = append(out, run{text: string(seg.Value(b.src)), style: st})
			}
		default:
			if tag, ok := n.(*mast.TagNode); ok {
				s := st
				s.bold = true
				out = append(out, run{text: "#" + string(tag.Tag), style: s})
				continue
			}
			if m, ok := n.(*mast.MentionNode); ok {
				out = append(out, run{text: "@" + string(m.Username), style: st})
				continue
			}
			if n.HasChildren() {
				out = append(out, b.inlines(n, st)...)
			}
		}
	}
	return out
}

func nodeText(n gast.Node, src []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *gast.Text:
			sb.Write(v.Segment.Value(src))
		case *gast.String:
			sb.Write(v.Value)
		default:
			sb.WriteString(nodeText(c, src))
		}
	}
	return sb.String()
}

func (b *builder) para(styleID, extraPPr string, runs []run) {
	b.body.WriteString(`<w:p><w:pPr><w:pStyle w:val="` + styleID + `"/>` + extraPPr + `</w:pPr>`)
	for _, r := range runs {
		b.writeRun(r)
	}
	b.body.WriteString(`</w:p>`)
}

func (b *builder) writeRun(r run) {
	if r.link != "" {
		b.rels = append(b.rels, r.link)
		fmt.Fprintf(&b.body, `<w:hyperlink r:id="rIdL%d">`, len(b.rels))
	}
	b.body.WriteString(`<w:r><w:rPr>`)
	if r.link != "" {
		b.body.WriteString(`<w:rStyle w:val="Hyperlink"/>`)
	}
	if r.code {
		b.body.WriteString(`<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas" w:cs="Consolas"/><w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/>`)
	}
	if r.bold {
		b.body.WriteString(`<w:b/>`)
	}
	if r.italic {
		b.body.WriteString(`<w:i/>`)
	}
	if r.strike {
		b.body.WriteString(`<w:strike/>`)
	}
	b.body.WriteString(`</w:rPr>`)
	if r.text != "" {
		parts := strings.Split(r.text, "\t")
		for i, p := range parts {
			if i > 0 {
				b.body.WriteString(`<w:tab/>`)
			}
			if p != "" {
				b.body.WriteString(`<w:t xml:space="preserve">` + escape(p) + `</w:t>`)
			}
		}
	}
	if r.br {
		b.body.WriteString(`<w:br/>`)
	}
	b.body.WriteString(`</w:r>`)
	if r.link != "" {
		b.body.WriteString(`</w:hyperlink>`)
	}
}

func escape(s string) string {
	var buf bytes.Buffer
	for _, c := range s {
		switch c {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&quot;")
		default:
			// Drop characters that are invalid in XML 1.0.
			if c == 0x9 || c == 0xA || c == 0xD || (c >= 0x20 && c <= 0xD7FF) || (c >= 0xE000 && c <= 0xFFFD) || (c >= 0x10000 && c <= 0x10FFFF) {
				buf.WriteRune(c)
			}
		}
	}
	return buf.String()
}

func (b *builder) writeZip(w io.Writer) error {
	zw := zip.NewWriter(w)
	var rels strings.Builder
	rels.WriteString(xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	rels.WriteString(`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	rels.WriteString(`<Relationship Id="rIdNumbering" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`)
	for i, target := range b.rels {
		fmt.Fprintf(&rels, `<Relationship Id="rIdL%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`, i+1, escape(target))
	}
	rels.WriteString(`</Relationships>`)
	files := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"word/document.xml", xmlHeader + `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` +
			b.body.String() + `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr></w:body></w:document>`},
		{"word/_rels/document.xml.rels", rels.String()},
		{"word/styles.xml", stylesXML},
		{"word/numbering.xml", numberingXML()},
	}
	for _, f := range files {
		fw, err := zw.Create(f.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(fw, f.body); err != nil {
			return err
		}
	}
	return zw.Close()
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

const contentTypes = xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/></Types>`

const rootRels = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`

func headingStyle(n, size int) string {
	return fmt.Sprintf(`<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="80"/><w:outlineLvl w:val="%d"/></w:pPr><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr></w:style>`, n, n, n-1, size)
}

var stylesXML = xmlHeader + `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Microsoft YaHei" w:cs="Calibri"/><w:sz w:val="22"/><w:lang w:val="en-US" w:eastAsia="zh-CN"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:qFormat/><w:rPr><w:b/><w:sz w:val="48"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Subtitle"><w:name w:val="Subtitle"/><w:basedOn w:val="Normal"/><w:qFormat/><w:rPr><w:color w:val="767676"/><w:sz w:val="18"/></w:rPr></w:style>` +
	headingStyle(1, 32) + headingStyle(2, 28) + headingStyle(3, 26) + headingStyle(4, 24) + headingStyle(5, 22) + headingStyle(6, 22) +
	`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:ind w:left="720"/></w:pPr><w:rPr><w:i/><w:color w:val="595959"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:pPr><w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/><w:spacing w:after="120" w:line="240" w:lineRule="auto"/></w:pPr><w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas" w:cs="Consolas"/><w:sz w:val="20"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="40"/></w:pPr></w:style>` +
	`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:rPr><w:color w:val="0563C1"/><w:u w:val="single"/></w:rPr></w:style>` +
	`</w:styles>`

func numberingXML() string {
	var sb strings.Builder
	sb.WriteString(xmlHeader + `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	for abs, fmtName := range []string{"bullet", "decimal"} {
		fmt.Fprintf(&sb, `<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="hybridMultilevel"/>`, abs)
		for lvl := 0; lvl < 9; lvl++ {
			lvlText := "•"
			if fmtName == "decimal" {
				lvlText = fmt.Sprintf("%%%d.", lvl+1)
			} else if lvl%2 == 1 {
				lvlText = "◦"
			}
			fmt.Fprintf(&sb, `<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`, lvl, fmtName, lvlText, 720*(lvl+1))
		}
		sb.WriteString(`</w:abstractNum>`)
	}
	sb.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num><w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num></w:numbering>`)
	return sb.String()
}

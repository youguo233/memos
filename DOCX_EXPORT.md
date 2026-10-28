# DOCX (Word) export

This fork adds Word (`.docx`) export to Memos. Conversion uses
[Pandoc](https://pandoc.org) (`pandoc -f gfm+hard_line_breaks -t docx`), the
mature open-source Markdown → Word converter, so you get real Word tables,
task lists, syntax-highlighted code blocks, hyperlinks and embedded images.
If the `pandoc` binary is not available, Memos logs a warning and falls back
to a small built-in pure-Go generator (`internal/docx`) with limited Markdown
support (tables as tab-separated text, no images).

## Using it

- **One memo** — open a memo's `⋮` action menu → **Export as Word (.docx)**.
- **All memos** — **Settings → Export & Import → Export to Word**. Optionally
  pick a start/end date (inclusive, in your browser's time zone), then click
  **Export .docx**. Each memo becomes a section headed by its created time,
  with its tags and visibility underneath. Archived memos and comments are
  not included (use the ZIP export for a complete backup).

### How conversion works (pandoc)

1. All selected memos are assembled into one GFM document: each memo is a
   Heading 1 with its created time, a small gray "Memo Meta" line with
   tags · visibility (· updated time; not italic, which looks bad for CJK), then the memo body (its own headings are demoted one
   level), with a thin light-gray separator between memos. The export title is set as
   the document title.
2. Images: `![](/file/attachments/{uid}/…)` links (relative or absolute URL)
   and image attachments of the memo that are not referenced inline are read
   from storage (database, local disk or S3) and inlined as `data:` URIs, so
   they are embedded in `word/media/`. Only attachments of that memo or of
   the memo's creator are resolved. Non-image attachments are listed by name.
   External `http(s)` images become links unless
   `MEMOS_DOCX_REMOTE_IMAGES=true`, in which case they are downloaded (8 s
   timeout, 20 MB cap) and skipped gracefully on failure. Any other image
   path becomes plain text.
3. pandoc runs with `--sandbox` (no file or network access besides the
   reference doc), Markdown on stdin, docx on stdout, 2-minute timeout tied to
   the request context. On error Memos falls back to the built-in generator.

### Configuration

| Env var | Meaning |
|---|---|
| `MEMOS_PANDOC_PATH` | pandoc binary (default: `pandoc` from `PATH`) |
| `MEMOS_DOCX_REFERENCE_DOC` | your own `reference.docx` for Word styles (fonts, heading styles, page size). Create one with `pandoc -o custom.docx --print-default-data-file reference.docx` and edit its styles in Word. |
| `MEMOS_DOCX_REMOTE_IMAGES` | `true` to download and embed external images |

The default reference doc (`server/memodocx/assets/reference.docx`, embedded
in the binary) is generated from pandoc's default by
`server/memodocx/assets/style_reference.py` (re-run it after changing styles):

- Fonts: Latin **Arial**, East Asian **PingFang SC** (Word stores one name per
  script; on Windows, where PingFang is absent, Word substitutes its default
  CJK font such as Microsoft YaHei/DengXian), code **Consolas** (bundled with
  Office on both Mac and Windows). zh-CN East Asian language.
- Body 10.5 pt, dark gray `#262626`, line spacing 1.45, 6 pt after.
- Memo time = Heading 1 at 13 pt bold, restrained navy `#1F3A5F`; memo-body
  headings near-black 12/11/10.5 pt bold. Meta line 9 pt gray `#8C8C8C`.
- Tables (`Table` style): 0.5 pt `#D9D9D9` grid, bold header row on
  `#EEF2F6`, subtle banded rows, cell padding; Memos post-processes pandoc's
  `document.xml` so tables span 100% of the text width.
- Code blocks: Consolas 9 pt on `#F6F7F9` with a hairline border; inline code
  shaded. Block quotes: gray left bar. Links `#2F6FB2`, no underline.

An embedded Lua filter (`assets/memos.lua`, written to a temp file, works with
`--sandbox`) applies the "Memo Meta" and "Memo Separator" styles.

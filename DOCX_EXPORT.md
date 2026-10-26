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
   Heading 1 with its created time, an italic line with tags · visibility
   (· updated time), then the memo body (its own headings are demoted one
   level), with a horizontal rule between memos. The export title is set as
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
in the binary) is pandoc's default with CJK-friendly fonts: Calibri / Microsoft
YaHei (East Asian), Consolas / SimSun for code, East Asian language zh-CN.
Word substitutes installed fonts when those are missing (e.g. on macOS).

## HTTP API

Authenticated with the usual `Authorization: Bearer <token>` (access token or
PAT) or the session cookie.

```
GET /file/memos/{uid}/docx?tz=Asia/Shanghai
GET /file/users/me/memos.docx?start=<unix>&end=<unix>&tz=Asia/Shanghai
```

`start` is inclusive and `end` exclusive (Unix seconds, both optional); `tz` is
an IANA zone for the printed times (default UTC). Single-memo export follows
the same read rules as the memo API (public memos are exportable anonymously
when the instance allows it).

```sh
curl -H "Authorization: Bearer $TOKEN" -o memos.docx \
  "http://localhost:8081/file/users/me/memos.docx?tz=Asia/Shanghai"
```

## Running locally

Requirements: Go (see `go.mod`), Node ≥ 24 and pnpm 11 (via corepack).

```sh
# build the frontend into the Go embed dir
cd web && corepack enable && pnpm install --frozen-lockfile && pnpm release && cd ..
# run the server (SQLite data in ./data)
mkdir -p data && go run ./cmd/memos --port 8081 --data ./data
# open http://localhost:8081
```

Install pandoc for full fidelity: `brew install pandoc`, `apt install pandoc`,
or the static binary from <https://github.com/jgm/pandoc/releases> (3.x
recommended; `--sandbox` needs ≥ 2.19).

Or with Docker — the image bundles pandoc's official static binary on amd64
and arm64 (arm/v7 has no static build and uses the built-in fallback). The Dockerfile embeds the prebuilt frontend, so run `pnpm release` first as above:

```sh
docker build -t memos-docx -f scripts/Dockerfile .
docker run -d -p 5230:5230 -v ~/.memos:/var/opt/memos memos-docx
```

Tests: `go test ./internal/docx/ ./server/...` (the pandoc integration test is
skipped when pandoc is not installed) and `cd web && pnpm lint`.

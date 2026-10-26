# DOCX (Word) export

This fork adds Word (`.docx`) export to Memos. Documents are generated in pure
Go (`internal/docx`, standard library + goldmark already used by Memos), so the
Docker image needs no pandoc, LibreOffice, or other runtime.

## Using it

- **One memo** — open a memo's `⋮` action menu → **Export as Word (.docx)**.
- **All memos** — **Settings → Export & Import → Export to Word**. Optionally
  pick a start/end date (inclusive, in your browser's time zone), then click
  **Export .docx**. Each memo becomes a section headed by its created time,
  with its tags and visibility underneath. Archived memos and comments are
  not included (use the ZIP export for a complete backup).

Markdown mapping: headings, paragraphs, bullet / numbered / task lists
(nested), block quotes, fenced code (monospace, shaded), **bold**, *italic*,
~~strike~~, `inline code`, links (real hyperlinks), `#tags` (bold), tables
(rows as tab-separated lines). Images are written as `[image: …]` links.

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

Or with Docker (the Dockerfile embeds the prebuilt frontend, so run `pnpm release` first as above):

```sh
docker build -t memos-docx -f scripts/Dockerfile .
docker run -d -p 5230:5230 -v ~/.memos:/var/opt/memos memos-docx
```

Tests: `go test ./internal/docx/ ./server/...` and `cd web && pnpm lint`.

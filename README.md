[![Gitter chat](https://badges.gitter.im/gitterHQ/gitter.png)](https://gitter.im/godocs/community) [![Go Report Card](https://goreportcard.com/badge/github.com/drummonds/godocs)](https://goreportcard.com/report/github.com/drummonds/godocs)

# godocs

![godocs screenshot](https://drummonds.github.io/godocs/screenshot.svg)
*Demo snapshot — [try the live demo](https://drummonds.github.io/godocs/demo/)*

A lightweight Electronic Document Management System (EDMS) for home users, built entirely in Go.

**Originally created by [deranjer/godocs](https://github.com/deranjer/goEDMS)** — hard fork with significant modernization.

## What is godocs?

A self-hosted document management system for scanning, organizing, and searching receipts, documents, and other files. Focused on **simplicity, speed, and reliability** over enterprise complexity.

## Quick Install

```bash
go install github.com/drummonds/godocs@latest
godocs
```

Starts with pglike (SQLite via go-postgres) — no external database needed.

## Key Design Principles

- **Easy Setup** — works out-of-the-box, no dependencies for basic use
- **Pure Go** — single binary with embedded static assets
- **Full-Text Search** — PostgreSQL tsvector or SQLite fallback
- **Step-Based Ingestion** — hash, deduplicate, OCR, index
- **Multiple Formats** — PDF, images (TIFF, JPG, PNG), text files

## Major Improvements in This Fork

- Go 1.22+ with structured logging (slog)
- SSR frontend using lofigui + pongo2 templates + Bulma CSS
- Pure Go PDF rendering (no CGo required)
- PostgreSQL full-text search with word cloud visualization
- Step-based ingestion with job tracking
- Canonical file naming with sidecar files (.ocr.txt, .thumb.png, .tags.json)

## Documentation

Full documentation: **[drummonds.github.io/godocs](https://drummonds.github.io/godocs/)**

- [Getting Started](https://drummonds.github.io/godocs/getting-started.html) — configuration, environment variables
- [Development](https://drummonds.github.io/godocs/development.html) — building, testing, deployment
- [API Reference](https://drummonds.github.io/godocs/api.html) — OpenAPI / Swagger
- [Internal Docs](https://drummonds.github.io/godocs/internal/file-naming.html) — file naming, tagging, lifecycle

## Links

| | |
|---|---|
| Documentation | https://h3-godocs.statichost.page/ |
| Source (Codeberg) | https://codeberg.org/hum3/godocs |
| Mirror (GitHub) | https://github.com/drummonds/godocs |

## License

See [LICENSE](LICENSE) file.

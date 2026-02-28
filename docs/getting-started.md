# Getting Started

## Quick Install

```bash
go install github.com/drummonds/godocs@latest
godocs
```

Starts with pglike (SQLite via go-postgres) by default — no external database needed. Open [http://localhost:8000](http://localhost:8000) in your browser.

## Configuration Modes

godocs supports three database modes:

### 1. pglike Mode (Simplest — No External Database)

```bash
DATABASE_TYPE=pglike ./godocs
```

Uses [go-postgres](https://github.com/drummonds/go-postgres) which translates PostgreSQL SQL to SQLite. The database is stored locally in a `databases/` directory. Supports all features except PostgreSQL full-text search (falls back to LIKE-based search).

### 2. Ephemeral PostgreSQL (Testing)

```bash
DATABASE_TYPE=ephemeral ./godocs
```

Starts with an ephemeral PostgreSQL database that is automatically created and destroyed when the application exits.

### 3. Local PostgreSQL with .env File (Production)

1. **Install and start PostgreSQL**
   ```bash
   # Ubuntu/Debian
   sudo apt install postgresql
   sudo systemctl start postgresql

   # macOS
   brew install postgresql
   brew services start postgresql
   ```

2. **Create a database and user**
   ```bash
   sudo -u postgres psql
   CREATE DATABASE godocs;
   CREATE USER godocs WITH PASSWORD 'your_password';
   GRANT ALL PRIVILEGES ON DATABASE godocs TO godocs;
   \q
   ```

3. **Configure .env file**
   ```bash
   # Production:
   sudo cp .env.example /etc/godocs.env
   sudo nano /etc/godocs.env

   # Local development:
   cp .env.example .env
   ```

4. **Edit configuration** with your database credentials:
   ```bash
   GODOCS_DATABASE_TYPE=postgres
   GODOCS_DATABASE_HOST=localhost
   GODOCS_DATABASE_PORT=5432
   GODOCS_DATABASE_NAME=godocs
   GODOCS_DATABASE_USER=godocs
   GODOCS_DATABASE_PASSWORD=your_password
   GODOCS_DATABASE_SSLMODE=disable
   ```

5. **Run godocs**
   ```bash
   ./godocs
   ```

## Configuration Priority

Database connection settings load in this order (later overrides earlier):

1. `/etc/godocs.env` file (production)
2. `.env` file (local development)
3. `config.env` file (alternative local config)
4. Environment variables (highest priority)

## Environment Variables

- `DATABASE_TYPE` — postgres, pglike, ephemeral, cockroachdb
- `DATABASE_HOST` — hostname (not needed for ephemeral)
- `DATABASE_PORT` — port (not needed for ephemeral)
- `DATABASE_NAME` — database name (not needed for ephemeral)
- `DATABASE_USER` — username (not needed for ephemeral)
- `DATABASE_PASSWORD` — password (not needed for ephemeral)
- `DATABASE_SSLMODE` — disable, require, etc.

See `.env.example` for a complete list.

## Technology Stack

- **Backend**: Go 1.22+ with Echo framework
- **Frontend**: SSR using lofigui + pongo2 templates + Bulma CSS
- **Database**: PostgreSQL with full-text search (tsvector), or SQLite via [go-postgres](https://github.com/drummonds/go-postgres)
- **OCR**: Tesseract for image and scanned PDF processing
- **PDF Processing**: Pure Go PDF rendering via PDFium WebAssembly (no CGo)
- **Job Tracking**: ULID-based job system with real-time progress

All application settings beyond the database connection are stored in the database, not in configuration files.

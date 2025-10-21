[![Gitter chat](https://badges.gitter.im/gitterHQ/gitter.png)](https://gitter.im/goEDMS/community) [![Go Report Card](https://goreportcard.com/badge/github.com/deranjer/goEDMS)](https://goreportcard.com/report/github.com/deranjer/goEDMS)
# goEDMS
A golang/react EDMS for home users.  This was originally created by https://github.com/deranjer/goEDMS

I am forking this as I want to develop it and to work on embedded Postgres,  My plan is to see if I can use it for my paperless use case where my old paperless app had failed and I wanted a version that I could eventualy
run on a gokrazy server
- Update to go 1.22
- Change logging to slog
- Remove imagemagick and replace with go libraries
- You really do need to have tesseract installed especially if none of your PDF's have text in them.


EDMS stands for Electronic Document Management System.  Essentially this is used to scan and organize all of your documents.  OCR is used to extract the text from PDF's, Images, or other formats that we cannot natively get text from.

goEDMS is an EDMS server for home users to scan in receipts/documents and search all the text in them.  The main focus of goEDMS is simplicity (to setup and use) as well as speed and reliability.  Less importance is placed on having very advanced features.

## Immediate roadmap

Have a very simple go-app that allows you to show documents and then click on them.  I want to reach feature
parity with the old react app and then to get rid of it.

- Add UI improvements
-- clean
  - iterate through and show progress
-- ingest and show progress
- move OR (tesseract) to a hosted service on podman which can host on gokrazy
- Add free text search
- Move to postgres embedded
- move to
- Add ingestion
- add tagging
- smart workflows for
  - Inbox
  - who
  - update
  - importance
  - search by tagging
  ==== Milestone deploy to gokrazy
  ==== Milestone can replace paperless for my use case
  -- Find a doc by text search/update ( )
  -- view doc on screen (x)
  -- print doc (x)
 ===== backup function and restore

- display summary using AI?
- archival moving docs to an archive


## Configuration

goEDMS supports multiple ways to configure the application:

### 1. Development Mode (Ephemeral PostgreSQL)

The easiest way to get started for development:

```bash
./goEDMS -dev
```

This starts goEDMS with an ephemeral PostgreSQL database that is automatically created and destroyed when the application exits. Perfect for testing and development!

### 2. Local PostgreSQL with .env File (Recommended)

For local development with a persistent PostgreSQL database:

1. **Install and start PostgreSQL** (if not already running)
   ```bash
   # On Ubuntu/Debian
   sudo apt install postgresql
   sudo systemctl start postgresql
   
   # On macOS with Homebrew
   brew install postgresql
   brew services start postgresql
   ```

2. **Create a database and user**
   ```bash
   sudo -u postgres psql
   CREATE DATABASE goedms;
   CREATE USER goedms WITH PASSWORD 'your_password';
   GRANT ALL PRIVILEGES ON DATABASE goedms TO goedms;
   \q
   ```

3. **Copy and configure .env file**
   ```bash
   cp .env.example .env
   ```

4. **Edit .env** with your database credentials:
   ```bash
   GOEDMS_DATABASE_TYPE=postgres
   GOEDMS_DATABASE_HOST=localhost
   GOEDMS_DATABASE_PORT=5432
   GOEDMS_DATABASE_NAME=goedms
   GOEDMS_DATABASE_USER=goedms
   GOEDMS_DATABASE_PASSWORD=your_password
   GOEDMS_DATABASE_SSLMODE=disable
   ```

5. **Run goEDMS**
   ```bash
   ./goEDMS
   ```

### 3. Traditional TOML Configuration

Edit `config/serverConfig.toml`:

```toml
[database]
    Type = "postgres"
    # Option 1: Full connection string
    ConnectionString = "host=localhost port=5432 user=goedms password=secret dbname=goedms sslmode=disable"
    
    # Option 2: Individual parameters
    Host = "localhost"
    Port = "5432"
    User = "goedms"
    Password = "secret"
    Name = "goedms"
    SSLMode = "disable"
```

### Configuration Priority

Settings are loaded in this order (later overrides earlier):

1. `config/serverConfig.toml` (default values)
2. `.env` file (if present)
3. Environment variables (highest priority)

This means you can set `GOEDMS_DATABASE_PASSWORD` as an environment variable to override the .env file value.

### Environment Variables

All configuration options can be set via environment variables using the prefix `GOEDMS_` and replacing dots with underscores:

- `GOEDMS_DATABASE_HOST` → `database.Host`
- `GOEDMS_SERVERCONFIG_SERVERPORT` → `serverConfig.ServerPort`
- `GOEDMS_INGRESS_INGRESSPATH` → `ingress.IngressPath`

See `.env.example` for a complete list of available variables.

## Documentation

[Documentation](https://deranjer.github.io/goEDMSDocs)


## Commands
Main Tasks:

**Development:**
- `task dev` - Run the backend application locally
- `task dev:full` - Run both backend and frontend together

**Testing:**
- `task test` - Run all Go tests
- `task test:coverage` - Run tests with coverage report (generates HTML)
- `task test:race` - Run tests with race detector

**Building:**
- `task build` - Build both frontend and backend
- `task build:backend` - Build only the backend

**Frontend:**
- `task frontend:install` - Install npm dependencies
- `task frontend:build` - Build the React frontend
- `task frontend:dev` - Run frontend dev server

**Dependencies:**
- `task deps:install` - Install all Go and npm dependencies
- `task deps:update` - Update all dependencies
- `task deps:tidy` - Tidy Go modules

**Code Quality:**
- `task fmt` - Format Go code
- `task vet` - Run go vet
- `task lint` - Run golangci-lint (if installed)
- `task check` - Run fmt, vet, and tests

**Cleanup:**
- `task clean` - Remove build artifacts
- `task clean:all` - Remove all generated files including node_modules

**Docker:**
- `task docker:build` - Build Docker image
- `task docker:run` - Run Docker container

## Quick Start:

1. Install Task: https://taskfile.dev/installation/
2. Run `task` or `task --list` to see all available tasks
3. Run `task dev` to start the application locally
4. Run `task test` to run tests

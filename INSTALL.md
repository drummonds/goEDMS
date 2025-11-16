# Installation Guide

## Quick Install (Recommended)

### Using go install

```bash
# Install latest version
go install github.com/drummonds/godocs@latest

# Verify installation
godocs --version
```

The binary will be installed to `$GOPATH/bin` (usually `~/go/bin`). Make sure this is in your PATH.

## Installation Methods

### Method 1: Pre-built Binaries (Easiest)

Download pre-built binaries from [GitHub Releases](https://github.com/drummonds/godocs/releases):

```bash
# Linux (x86_64)
wget https://github.com/drummonds/godocs/releases/latest/download/godocs_Linux_x86_64.tar.gz
tar -xzf godocs_Linux_x86_64.tar.gz
sudo mv godocs /usr/local/bin/

# macOS (ARM64 - M1/M2)
wget https://github.com/drummonds/godocs/releases/latest/download/godocs_Darwin_arm64.tar.gz
tar -xzf godocs_Darwin_arm64.tar.gz
sudo mv godocs /usr/local/bin/

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/drummonds/godocs/releases/latest/download/godocs_Windows_x86_64.zip" -OutFile "godocs.zip"
Expand-Archive -Path godocs.zip -DestinationPath .
# Move godocs.exe to desired location
```

### Method 2: Build from Source

#### Prerequisites
- Go 1.21 or later
- [Task](https://taskfile.dev/) (optional but recommended)
- Git

#### Using Task (Recommended)

```bash
# Clone repository
git clone https://github.com/drummonds/godocs.git
cd godocs

# Build everything (backend + WASM frontend)
task build

# Binary will be in build/godocs
./build/godocs
```

#### Using go build

```bash
# Clone repository
git clone https://github.com/drummonds/godocs.git
cd godocs

# Build WASM frontend first
task build:wasm
# Or manually:
# GOOS=js GOARCH=wasm go build -o web/app.wasm ./cmd/webapp

# Build backend with version
VERSION=$(git describe --tags --always)
go build -ldflags="-X 'github.com/drummonds/godocs/internal/build.Version=${VERSION}'" -o godocs main.go

# Run
./godocs
```

### Method 3: Development Setup

```bash
# Clone repository
git clone https://github.com/drummonds/godocs.git
cd godocs

# Install dependencies
go mod download

# Run with auto-reload (requires task)
task dev:full
```

## Version Information

### Check Your Version

```bash
godocs --version
# Or start the server and check logs:
godocs 2>&1 | grep version
```

### Version Formats

- **Release builds**: `v1.2.3` (from git tags)
- **Development builds**: `v1.2.3-5-gabc1234` (tag + commits + hash)
- **go install builds**: `v0.0.0-abc1234` (git hash) or tag if available
- **Dirty working tree**: Adds `+dirty` suffix

## Creating Releases

### For Maintainers

#### Using GoReleaser (Recommended)

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Create a tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Build release (requires GITHUB_TOKEN)
export GITHUB_TOKEN="your_token_here"
goreleaser release

# Or test locally
goreleaser release --snapshot --clean
```

#### Manual Release

```bash
# Tag the release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Build for multiple platforms
task build              # Linux binary
task build:wasm         # WebAssembly app

# Create archives manually and upload to GitHub
```

## Post-Installation

### First Run

```bash
# Run godocs
godocs

# By default, it will:
# - Start on http://localhost:8000
# - Use SQLite database (in current directory)
# - Create necessary directories (ingress, documents)
```

### Configuration

Copy `.env.example` to `.env` and customize:

```bash
cp .env.example .env
# Edit .env with your settings
```

See [README.md](README.md) for configuration options.

## Troubleshooting

### "command not found: godocs"

Make sure `$GOPATH/bin` is in your PATH:

```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Version shows "development"

This means the binary wasn't built with version info. Use `task build` or set ldflags manually.

### WASM app not loading

Make sure you built the WASM app before starting:

```bash
task build:wasm
```

## Uninstall

```bash
# If installed via go install
rm $(go env GOPATH)/bin/godocs

# If installed system-wide
sudo rm /usr/local/bin/godocs
```

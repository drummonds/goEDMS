# godocs Architecture - Frontend/Backend Separation

## Overview

godocs now supports three deployment modes:

1. **Combined Mode** (default) - Frontend and backend run together on one server
2. **Backend-Only Mode** - Standalone API server
3. **Frontend-Only Mode** - Standalone WASM app server with API proxy

This architecture follows the Backend-for-Frontend (BFF) pattern, allowing independent development, testing, and deployment of frontend and backend components.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        Combined Mode                         │
│                     ./godocs (port 8000)                     │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Frontend HTTP + Backend API (same server)       │   │
│  │  - All routes on one port                            │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘

```

## Deployment Modes

### 1. Combined Mode (Backwards Compatible)

**Use when:** Simple deployment, development, or small-scale usage

```bash
# Run everything together
./godocs

# Access at http://localhost:8000
```

**Benefits:**
- Single binary to deploy
- Simplest configuration
- No CORS issues
- Backwards compatible with existing setups

**Files:**
- Binary: `./godocs`
- Source: `main.go`
- Config: `config.env` or `.env`

---

### 2. Backend-Only Mode

**Use when:**
- Testing backend APIs independently
- Multiple frontends consuming same backend
- Microservices architecture
- Separate scaling of backend

```bash
# Start backend API server
./godocs-backend --port 8000

# Or with ephemeral database
DATABASE_TYPE=ephemeral ./godocs-backend
```

**Configuration:**
```bash
# Copy example config
cp backend.env.example backend.env

# Edit backend.env
nano backend.env
```

**Key Config Options:**
- `SERVER_PORT`: API server port (default: 8000)
- `DATABASE_TYPE`: postgres or cockroachdb
- `POSTGRES_*`: Database connection settings
- `DOCUMENT_PATH`: Document storage location
- `TESSERACT_PATH`: OCR executable path
- `INGRESS_PATH`: Document ingestion folder

**API Endpoints:**
All endpoints are under `/api/*`:
- `GET /api/health` - Health check
- `GET /api/documents/latest` - Recent documents
- `GET /api/search?term=...` - Search documents
- `GET /api/wordcloud` - Word cloud data
- `POST /api/ingest` - Trigger ingestion
- And many more...

**Benefits:**
- Independent testing of backend logic
- Can be deployed to different infrastructure
- Easy to add authentication/rate limiting
- Multiple frontend can share one backend

**Files:**
- Binary: `./godocs-backend`
- Source: `cmd/backend/main.go`
- Config: `backend.env`

---

### 3. Frontend-Only Mode

**Use when:**
- Frontend development without running full backend
- Connecting to remote backend API
- Static hosting with API proxy
- Separate scaling of frontend

```bash
# Start frontend server (proxies to backend)
./godocs-frontend --port 3000 --api http://localhost:8000

# Or use config file
cp frontend.env.example frontend.env
./godocs-frontend
```

**Configuration:**
```bash
# frontend.env
SERVER_API_URL=http://localhost:8000  # Backend API URL
NEW_DOCUMENT_COUNT=5
LOG_LEVEL=info
```

**How it Works:**
1. Frontend server serves WASM app and static files
2. Injects backend API URL as `window.godocsConfig.apiURL`
3. WASM app reads config and makes API calls to backend
4. Frontend server proxies `/api/*` requests to backend
5. Handles CORS automatically

**Benefits:**
- Frontend development with hot-reload
- Can point to different backend environments
- Test frontend against production API
- Deploy frontend to CDN

**Files:**
- Binary: `./godocs-frontend`
- Source: `cmd/frontend/main.go`
- Config: `frontend.env`

---

## API Endpoint Organization

All backend JSON APIs are under the `/api/*` namespace:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/health` | GET | Health check |
| `/api/documents/latest` | GET | Recent documents |
| `/api/documents/filesystem` | GET | File tree |
| `/api/document/:id` | GET | Get document |
| `/api/document/*` | DELETE | Delete document |
| `/api/document/move/*` | PATCH | Move document |
| `/api/document/upload` | POST | Upload document |
| `/api/folder/:folder` | GET | Get folder |
| `/api/folder/*` | POST | Create folder |
| `/api/search` | GET | Search documents |
| `/api/search/reindex` | POST | Reindex search |
| `/api/ingest` | POST | Trigger ingestion |
| `/api/clean` | POST | Clean database |
| `/api/about` | GET | System information |
| `/api/wordcloud` | GET | Word cloud data |
| `/api/wordcloud/recalculate` | POST | Recalculate word cloud |

Document view routes (serve actual files): `/document/view/:ulid`

---

## Development Workflows

### Full Stack Development
```bash
# Terminal 1: Run backend with ephemeral DB
DATABASE_TYPE=ephemeral ./godocs-backend --port 8000

# Terminal 2: Run frontend pointing to backend
./godocs-frontend --port 3000 --api http://localhost:8000

# Access app at http://localhost:3000
# API at http://localhost:8000/api
```

### Backend API Development
```bash
# Run backend only
DATABASE_TYPE=ephemeral ./godocs-backend

# Test API with curl
curl http://localhost:8000/api/health
curl http://localhost:8000/api/documents/latest

# Run tests
go test ./engine/... -v
go test ./database/... -v
```

### Frontend Development
```bash
# Use existing backend (or mock)
./godocs-frontend --api http://remote-backend:8000

# Rebuild WASM when changing frontend code
GOOS=js GOARCH=wasm go build -o web/app.wasm ./cmd/webapp

# Frontend automatically picks up new WASM
```

---

## Configuration Files

### Backend Configuration (`backend.env`)
- Database settings
- Document storage paths
- OCR/Tesseract configuration
- Ingestion settings
- Authentication (if enabled)

### Frontend Configuration (`frontend.env`)
- Backend API URL
- UI preferences
- Feature flags (future)

### Combined Mode (`.env` or `config.env`)
- Contains both backend and frontend settings
- Used by original `./godocs` binary

---

## Testing Strategy

### Unit Tests
```bash
# Test backend logic
go test ./database/... -v
go test ./engine/... -v
go test ./config/... -v

# Test frontend components
go test ./webapp/... -v
```

### Integration Tests
```bash
# Test full application
go test -v .

# Test with ephemeral database
DATABASE_TYPE=ephemeral go test -v .
```

### API Testing
```bash
# Start backend
DATABASE_TYPE=ephemeral ./godocs-backend &

# Run API tests
./test-api.sh  # (create your own test script)
```

---

## Deployment Examples

### Docker Compose
```yaml
version: '3.8'
services:
  backend:
    build:
      context: .
      dockerfile: Dockerfile.backend
    ports:
      - "8000:8000"
    environment:
      - DATABASE_TYPE=postgres
      - POSTGRES_HOST=db
    depends_on:
      - db

  frontend:
    build:
      context: .
      dockerfile: Dockerfile.frontend
    ports:
      - "3000:3000"
    environment:
      - SERVER_API_URL=http://backend:8000

  db:
    image: postgres:15
    environment:
      - POSTGRES_DB=godocs
      - POSTGRES_USER=godocs
      - POSTGRES_PASSWORD=postgres
```

### Single Server (Combined)
```bash
# Simple deployment
./godocs

# With systemd service
sudo systemctl start godocs
```

### Kubernetes (Separated)
```yaml
# Backend Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: godocs-backend
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: backend
        image: godocs-backend:latest
        ports:
        - containerPort: 8000

---
# Frontend Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: godocs-frontend
spec:
  replicas: 5
  template:
    spec:
      containers:
      - name: frontend
        image: godocs-frontend:latest
        env:
        - name: SERVER_API_URL
          value: "http://godocs-backend:8000"
```

---

## Migration Guide

### From Existing Installation

Your existing setup continues to work! The `./godocs` binary functions identically to before.

To migrate to separated mode:

1. **Keep current setup running**
   ```bash
   ./godocs  # Still works!
   ```

2. **Try backend-only** (optional)
   ```bash
   ./godocs-backend  # Uses same database and config
   ```

3. **Try frontend-only** (optional)
   ```bash
   # Point to your running backend
   ./godocs-frontend --api http://localhost:8000
   ```

4. **Gradual migration**
   - Start by separating in development
   - Move to separated production when ready
   - No database migration needed

---

## Benefits of Separation

### Development
- ✅ Independent frontend/backend development
- ✅ Faster frontend iteration (no backend rebuild)
- ✅ Easier testing of each component
- ✅ Multiple developers can work in parallel

### Deployment
- ✅ Scale frontend and backend independently
- ✅ Deploy frontend to CDN
- ✅ Backend can run on more powerful hardware
- ✅ Multiple frontends can share backend

### Maintenance
- ✅ Clear separation of concerns
- ✅ Easier to debug issues
- ✅ Better security boundaries
- ✅ Simpler to add authentication layers

---

## Troubleshooting

### Frontend can't connect to backend
```bash
# Check backend is running
curl http://localhost:8000/api/health

# Check frontend config
./godocs-frontend --api http://localhost:8000

# Check CORS is enabled (backend has it by default)
```

### API calls fail with CORS errors
- Backend enables CORS by default for separated mode
- Check browser console for CORS messages
- Ensure `SERVER_API_URL` matches your backend URL

### WASM app not loading backend config
- Check `/config.js` is being served
- Open browser dev tools → Network → check for config.js
- Check console for "godocs Config loaded" message

---

## Files Reference

| File | Purpose |
|------|---------|
| `main.go` | Combined mode server |
| `cmd/backend/main.go` | Backend-only server |
| `cmd/frontend/main.go` | Frontend-only server |
| `webapp/api.go` | API URL helper functions |
| `config/config.go` | Configuration loading |
| `backend.env.example` | Backend config template |
| `frontend.env.example` | Frontend config template |

---

## Next Steps

1. Try running backend and frontend separately in development
2. Create your own deployment configuration
3. Set up monitoring for separated services
4. Consider adding authentication layer to backend
5. Deploy frontend to CDN for better performance

For questions or issues, please file a GitHub issue.

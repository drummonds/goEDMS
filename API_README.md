# goEDMS API Documentation

## Overview

goEDMS provides a comprehensive REST API for document management operations. The API supports document upload, retrieval, search, and folder management.

## API Specification

A complete OpenAPI 3.0 specification is available at `api/openapi.yaml`. This specification can be used with tools like Swagger UI, Postman, or any OpenAPI-compatible client.

## Base URL

- Local Development: `http://localhost:8000`
- Production: Configure via `ListenAddrPort` in configuration

## Authentication

Currently, goEDMS does not implement authentication. This should be added before production deployment.

## API Endpoints

### Documents

#### Get Latest Documents
```
GET /home
```
Returns the most recently ingested documents with pagination support.

**Query Parameters**:
- `page` (optional): Page number (default: 1)

**Response**: Paginated response with metadata
```json
{
  "documents": [...],
  "page": 1,
  "pageSize": 20,
  "totalCount": 100,
  "totalPages": 5,
  "hasNext": true,
  "hasPrevious": false
}
```

**Example**:
```bash
# Get first page
curl http://localhost:8000/home

# Get specific page
curl http://localhost:8000/home?page=2
```

#### Get Document Filesystem
```
GET /documents/filesystem
```
Returns the complete document tree structure.

**Response**: FileSystem object with hierarchical file tree

**Example**:
```bash
curl http://localhost:8000/documents/filesystem
```

#### Get Document by ID
```
GET /document/:id
```
Returns a specific document by its ULID.

**Parameters**:
- `id` (path): Document ULID

**Example**:
```bash
curl http://localhost:8000/document/01K7WTQXY83JPQRHTXEADHQW4V
```

#### View/Download Document
```
GET /document/view/:ulid
```
Direct access to view or download a document file.

**Parameters**:
- `ulid` (path): Document ULID

**Example**:
```bash
curl http://localhost:8000/document/view/01K7WTQXY83JPQRHTXEADHQW4V > document.pdf
```

#### Upload Document
```
POST /document/upload
```
Uploads a new document to the ingress folder for processing.

**Content-Type**: `multipart/form-data`

**Parameters**:
- `file` (form): The file to upload
- `path` (form): Relative path where to store the file (optional)

**Example**:
```bash
curl -X POST http://localhost:8000/document/upload \
  -F "file=@document.pdf" \
  -F "path=invoices/"
```

#### Delete Document
```
DELETE /document/?id={id}&path={path}
```
Deletes a file or folder from the database and filesystem.

**Parameters**:
- `id` (query): Document ULID
- `path` (query): Relative path to the file/folder

**Example**:
```bash
curl -X DELETE "http://localhost:8000/document/?id=01K7WTQXY83JPQRHTXEADHQW4V&path=document.pdf"
```

#### Move Documents
```
PATCH /document/move/?folder={folder}&id={id}
```
Moves one or more documents to a new folder.

**Parameters**:
- `folder` (query): Target folder path
- `id` (query): Document ULID(s) - can be repeated for multiple documents

**Example**:
```bash
curl -X PATCH "http://localhost:8000/document/move/?folder=/archive&id=01K7WTQXY83JPQRHTXEADHQW4V"
```

### Search

#### Search Documents
```
GET /search/?term={searchTerm}
```
Full-text search across all documents using OCR-extracted text.

**Parameters**:
- `term` (query): Search term or phrase

**Response**: FileSystem object with search results

**Example**:
```bash
curl "http://localhost:8000/search/?term=invoice"
```

### Folders

#### Get Folder Contents
```
GET /folder/:folder
```
Returns all documents in a specific folder.

**Parameters**:
- `folder` (path): Folder name/path

**Example**:
```bash
curl http://localhost:8000/folder/invoices
```

#### Create Folder
```
POST /folder/?path={path}&folder={folderName}
```
Creates a new folder in the document tree.

**Parameters**:
- `folder` (query): Folder name to create
- `path` (query): Parent path where to create folder

**Example**:
```bash
curl -X POST "http://localhost:8000/folder/?path=/documents&folder=archive"
```

### Admin Operations

#### Trigger Manual Ingestion
```
POST /api/ingest
```
Manually triggers the document ingestion process to scan the ingress folder and process any pending documents.

**Response**: Status message

**Example**:
```bash
curl -X POST http://localhost:8000/api/ingest
```

**Response**:
```
Ingestion started
```

#### Clean Database
```
POST /api/clean
```
Performs database cleanup operations:
- Removes database entries for files that no longer exist
- Moves orphaned files (not in database) back to ingress for reprocessing
- Removes orphaned entries from search index

**Response**: Cleanup statistics

**Example**:
```bash
curl -X POST http://localhost:8000/api/clean
```

**Response**:
```json
{
  "message": "Cleanup completed successfully",
  "scanned": 150,
  "deleted": 5,
  "moved": 2
}
```

## Data Models

### Document
```json
{
  "StormID": 1,
  "Name": "invoice_2024.pdf",
  "Path": "/home/user/goEDMS/documents/invoice_2024.pdf",
  "IngressTime": "2025-10-19T00:33:40.936452334+01:00",
  "Folder": "/home/user/goEDMS/documents",
  "Hash": "4f32ce0b88869c473aff5f4678f1fbb3",
  "ULID": "01K7WTQXY83JPQRHTXEADHQW4V",
  "DocumentType": ".pdf",
  "FullText": "OCR extracted text content...",
  "URL": "/document/view/01K7WTQXY83JPQRHTXEADHQW4V"
}
```

### FileTreeNode
```json
{
  "id": "01K7WTQXY83JPQRHTXEADHQW4V",
  "ulid": "01K7WTQXY83JPQRHTXEADHQW4V",
  "name": "invoice_2024.pdf",
  "size": 110963,
  "modDate": "2025-10-19 00:33:40.946496508 +0100 BST",
  "openable": true,
  "parentID": "01K7XWQS813KRPCQ3RR9FC2WVNdocuments",
  "isDir": false,
  "childrenIDs": null,
  "fullPath": "/home/user/goEDMS/documents/invoice_2024.pdf",
  "fileURL": "/document/view/01K7WTQXY83JPQRHTXEADHQW4V"
}
```

### FileSystem
```json
{
  "fileSystem": [
    {
      "id": "root",
      "name": "documents",
      "isDir": true,
      "childrenIDs": ["file1", "file2"],
      ...
    },
    ...
  ],
  "error": ""
}
```

## User Interfaces

### go-app Web UI (Default)
The default UI is a Progressive Web App (PWA) built with the go-app framework. This provides a native-like experience with offline capability and fast performance.

**Base URL**: http://localhost:8000/

**Available Routes**:
- `/` - Home page with latest documents (paginated)
- `/browse` - Browse documents in tree view
- `/search` - Full-text search interface
- `/ingest` - Document upload interface
- `/clean` - Database cleanup and maintenance

**Features**:
- Progressive Web App (PWA) with offline support
- Responsive design for mobile and desktop
- File browser with tree view
- Document upload with drag-and-drop
- Full-text search across all documents
- Document viewer
- Folder management
- Pagination for large document collections
- Real-time database cleanup tools

**Technical Components** (in `webapp/` directory):
- `app.go` - Main application component with routing
- `handler.go` - HTTP handler configuration
- `homepage.go` - Latest documents view with pagination
- `browsepage.go` - File tree browser
- `searchpage.go` - Search interface
- `ingestpage.go` - Document upload interface
- `cleanpage.go` - Database cleanup interface
- `navbar.go` - Top navigation component
- `sidebar.go` - Side navigation component

## Configuration

The API behavior is configured through `config.toml`:

```toml
[serverConfig]
ListenAddrIP = ""  # Bind to all interfaces
ListenAddrPort = "8000"
DocumentPath = "./documents"
IngressPath = "./ingress"
IngressInterval = 10  # minutes
TesseractPath = "/usr/bin/tesseract"  # OCR engine
DatabaseType = "postgres"  # "postgres", "cockroachdb", or "sqlite"

[database]
# Option 1: Provide a full connection string
ConnectionString = "postgresql://user:password@localhost:5432/goedms?sslmode=disable"

# Option 2: Provide individual components (used if ConnectionString is not set)
Host = "localhost"
Port = "5432"
User = "goedms"
Password = "your_password"
Name = "goedms"
SSLMode = "disable"  # "disable", "require", "verify-ca", or "verify-full"
```

**Database Configuration Notes**:
- PostgreSQL is now the default database (previously used BoltDB)
- Use `-dev` flag to run with ephemeral PostgreSQL (no persistent data)
- Connection string takes precedence over individual components
- For production, use proper SSL configuration

## Error Handling

The API uses standard HTTP status codes:

- `200 OK` - Successful request
- `204 No Content` - Successful request with no content (e.g., empty search)
- `400 Bad Request` - Invalid request parameters
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

Error responses include a JSON message:
```json
{
  "message": "Error description"
}
```

## CORS

CORS is enabled by default with `middleware.DefaultCORSConfig`, allowing cross-origin requests from any domain. Configure this appropriately for production use.

## Future Enhancements

- [ ] Authentication and authorization
- [ ] Rate limiting
- [ ] WebSocket support for real-time updates
- [ ] Batch operations
- [ ] Document versioning
- [ ] Advanced search filters (date ranges, file types, etc.)
- [ ] Document metadata editing
- [ ] API key management
- [ ] Audit logging
- [ ] Document tagging and categories
- [ ] Email integration for document ingestion
- [ ] Automated backup and restore functionality

## Development

### Running Tests
```bash
go test -v
```

### Building
```bash
go build -o goedms .
```

### Running

**Production Mode** (uses configured database):
```bash
./goedms
```

**Development Mode** (uses ephemeral PostgreSQL):
```bash
./goedms -dev
```
Development mode starts an ephemeral PostgreSQL database that is destroyed on exit. Perfect for testing without affecting persistent data.

### Building the go-app UI
The go-app UI is compiled to WebAssembly. To rebuild:
```bash
cd webapp
GOARCH=wasm GOOS=js go build -o ../web/app.wasm ./cmd/webapp
```

## Contributing

### Adding New API Endpoints

1. Add route definition in `main.go`
2. Implement handler in `engine/routes.go`
3. Update OpenAPI specification in `api/openapi.yaml` (if exists)
4. Add tests in `main_test.go`
5. Update this README

### Adding New UI Pages (go-app)

1. Create new page component in `webapp/` (e.g., `newpage.go`)
2. Add route in `webapp/handler.go`
3. Add route case in `webapp/app.go` `renderPage()` method
4. Update navigation in `webapp/navbar.go` or `webapp/sidebar.go`
5. Rebuild WebAssembly: `GOARCH=wasm GOOS=js go build -o ../web/app.wasm ./cmd/webapp`

## See Also

- [OpenAPI Specification](api/openapi.yaml) - Complete API specification
- [Main README](README.md) - Project overview and setup instructions

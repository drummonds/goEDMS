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
Returns the most recently ingested documents.

**Response**: Array of Document objects

**Example**:
```bash
curl http://localhost:8000/home
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

### React UI (Default)
The default UI is a React-based single-page application located at the root path `/`.

**Access**: http://localhost:8000/

**Features**:
- File browser with tree view
- Document upload with drag-and-drop
- Full-text search
- Document viewer
- Folder management

### go-app UI (Alternative - In Development)
An alternative Progressive Web App (PWA) built with go-app framework is in development.

**Location**: `/app` (currently under development)

**Planned Features**:
- Native-like performance
- Offline capability
- Home page with latest documents
- Browse documents in tree view
- Full-text search

**Components** (in `webapp/` directory):
- `homepage.go` - Latest documents view
- `browsepage.go` - File tree browser
- `searchpage.go` - Search interface
- `navbar.go` - Navigation component

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
```

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
- [ ] Advanced search filters
- [ ] Document metadata editing
- [ ] Complete go-app UI implementation
- [ ] API key management
- [ ] Audit logging

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
```bash
./goedms
```

## Contributing

When adding new API endpoints:

1. Add route definition in `main.go`
2. Implement handler in `engine/routes.go`
3. Update OpenAPI specification in `api/openapi.yaml`
4. Add tests in `main_test.go`
5. Update this README

## See Also

- [OpenAPI Specification](api/openapi.yaml) - Complete API specification
- [Main README](README.md) - Project overview and setup instructions

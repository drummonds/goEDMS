# Test Document Generator

A command-line tool to generate test PDF documents and their corresponding .txt sidecar files for testing godocs functionality.

## Usage

```bash
# Run the generator
go run cmd/testgen/main.go

# Or build and run
go build -o testgen cmd/testgen/main.go
./testgen
```

## Generated Test Documents

The tool creates test documents in the `testdocs/` directory:

### 1. Empty PDF (`1-empty.pdf`)
- **Purpose**: Test handling of PDFs with no text content
- **Content**: Empty PDF with a single blank page
- **Sidecar**: Empty `1-empty.txt` file

### 2. Hello World PDF (`2-hello.pdf`)
- **Purpose**: Test basic text extraction
- **Content**: Simple "Hello World" text with a short description
- **Sidecar**: `2-hello.txt` with the same text content

### 3. Diagram PDF (`3-diagram.pdf`)
- **Purpose**: Test handling of PDFs with graphics/diagrams
- **Content**: System architecture flowchart diagram
- **Sidecar**: `3-diagram.txt` with description of the diagram

### 4. Long Text PDF (`4-longtext.pdf`)
- **Purpose**: Test handling of documents with large amounts of text
- **Content**: Go source code example (demonstrates code formatting)
- **Sidecar**: `4-longtext.txt` with the same source code

## Use Cases

- **Ingestion Testing**: Copy files to the ingress folder to test document ingestion
- **OCR Testing**: Test OCR functionality with different document types
- **Sidecar Testing**: Verify sidecar .txt file reading and writing
- **Search Testing**: Test full-text search with various content types
- **Performance Testing**: Generate multiple documents for load testing

## Sidecar .txt Files

Each PDF has a corresponding `.txt` file that contains:
- For text-based PDFs: The actual text content
- For diagrams: A description of the diagram
- For empty PDFs: An empty file

This allows testing the `USE_SIDECAR_TXT` feature where godocs uses pre-existing .txt files instead of running OCR.

## Directory Structure

```
testdocs/
├── 1-empty.pdf
├── 1-empty.txt
├── 2-hello.pdf
├── 2-hello.txt
├── 3-diagram.pdf
├── 3-diagram.txt
├── 4-longtext.pdf
└── 4-longtext.txt
```

**Note**: The `testdocs/` directory is git-ignored and will not be committed to version control.

## Dependencies

- `github.com/jung-kurt/gofpdf` - PDF generation library

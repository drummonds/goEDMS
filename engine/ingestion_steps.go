package engine

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/drummonds/godocs/database"
	"github.com/oklog/ulid/v2"
)

// IngestDocumentWithSteps processes a document through explicit steps with progress tracking
// Step 1: Calculate hash and create initial database record
// Step 2: Move file to documents folder and verify hash
// Step 3: Extract text and update search/wordcloud
func (serverHandler *ServerHandler) IngestDocumentWithSteps(filePath string, db database.Repository, jobID ulid.ULID, fileNum, totalFiles int) error {
	fileName := filepath.Base(filePath)
	baseProgress := int((float64(fileNum) / float64(totalFiles)) * 90) // Reserve 90% for file processing, 10% for final steps

	// Step 1: Calculate hash and check for duplicates
	stepMsg := fmt.Sprintf("[%d/%d] %s - Step 1: Calculating hash", fileNum+1, totalFiles, fileName)
	db.UpdateJobProgress(jobID, baseProgress, stepMsg)
	Logger.Info("Step 1: Calculating hash", "filePath", filePath)

	fileHash, err := calculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("step 1 failed (hash calculation): %w", err)
	}

	// Check for duplicates
	duplicate, existingDoc := serverHandler.checkDuplicate(fileHash, fileName, db)
	if duplicate {
		Logger.Info("Duplicate document detected, skipping", "fileName", fileName, "existingDoc", existingDoc.Name)
		// Delete the duplicate source file
		if err := os.Remove(filePath); err != nil {
			Logger.Error("Failed to remove duplicate file", "filePath", filePath, "error", err)
		}
		return fmt.Errorf("duplicate document (hash: %s)", fileHash)
	}

	// Create initial database record with hash
	doc, err := serverHandler.createInitialDocument(filePath, fileHash, db)
	if err != nil {
		return fmt.Errorf("step 1 failed (create record): %w", err)
	}

	Logger.Info("Step 1 complete: Document record created", "ulid", doc.ULID.String(), "hash", fileHash)

	// Step 2: Move file and verify hash
	stepMsg = fmt.Sprintf("[%d/%d] %s - Step 2: Moving file", fileNum+1, totalFiles, fileName)
	db.UpdateJobProgress(jobID, baseProgress+10, stepMsg)
	Logger.Info("Step 2: Moving file to documents folder", "from", filePath, "to", doc.Path)

	err = serverHandler.moveAndVerifyFile(filePath, doc.Path, fileHash)
	if err != nil {
		// Rollback: delete the database record
		db.DeleteDocument(doc.ULID.String())
		return fmt.Errorf("step 2 failed (move/verify): %w", err)
	}

	Logger.Info("Step 2 complete: File moved and hash verified", "path", doc.Path)

	// Step 3: Extract text and update database
	// NOTE: This step should NEVER fail - if text extraction fails, we store the document without text
	stepMsg = fmt.Sprintf("[%d/%d] %s - Step 3: Extracting text", fileNum+1, totalFiles, fileName)
	db.UpdateJobProgress(jobID, baseProgress+20, stepMsg)
	Logger.Info("Step 3: Extracting text and updating search", "filePath", doc.Path)

	fullText, err := serverHandler.extractText(doc.Path)
	if err != nil {
		Logger.Warn("Text extraction failed, storing document without text", "error", err, "fileName", fileName)
		fullText = "" // Store document even if text extraction fails
	}

	// Update document with full text - if this fails, log error but don't fail the ingestion
	err = serverHandler.updateDocumentText(doc, fullText, db)
	if err != nil {
		Logger.Error("Failed to update document text, but document is still saved", "error", err, "ulid", doc.ULID.String())
		// Don't return error - the document record and file already exist, which is the important part
	}

	// Add document view route (skip if Echo is not initialized, e.g., during testing)
	documentURL := "/document/view/" + doc.ULID.String()
	if serverHandler.Echo != nil {
		serverHandler.Echo.File(documentURL, doc.Path)
	}
	_, err = database.UpdateDocumentField(doc.ULID.String(), "URL", documentURL, db)
	if err != nil {
		Logger.Error("Unable to update document URL field", "error", err, "ulid", doc.ULID.String())
		// Don't fail - this is not critical
	}

	Logger.Info("Step 3 complete: Text extracted and indexed", "textLength", len(fullText), "fileName", fileName)
	Logger.Info("Document ingestion complete", "fileName", fileName, "ulid", doc.ULID.String())

	return nil
}

// calculateFileHash computes MD5 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// checkDuplicate checks if a document with the same hash already exists
func (serverHandler *ServerHandler) checkDuplicate(fileHash string, fileName string, db database.Repository) (bool, *database.Document) {
	document, err := db.GetDocumentByHash(fileHash)
	if err != nil || document == nil {
		return false, nil
	}
	Logger.Info("Duplicate document found", "fileName", fileName, "existingDocument", document.Name, "hash", fileHash)
	return true, document
}

// createInitialDocument creates a minimal document record with hash
func (serverHandler *ServerHandler) createInitialDocument(filePath string, fileHash string, db database.Repository) (*database.Document, error) {
	serverConfig, err := database.FetchConfigFromDB(db)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch config: %w", err)
	}

	newTime := time.Now()
	newULID, err := database.CalculateUUID(newTime)
	if err != nil {
		return nil, fmt.Errorf("cannot generate ULID: %w", err)
	}

	doc := &database.Document{
		Name:         filepath.Base(filePath),
		Hash:         fileHash,
		IngressTime:  newTime,
		ULID:         newULID,
		DocumentType: filepath.Ext(filePath),
		FullText:     "", // Will be populated in step 3
	}

	// Calculate destination path
	if serverConfig.IngressPreserve {
		basePath := serverConfig.IngressPath
		newFileNameRoot := serverConfig.DocumentPath
		relativePath, err := filepath.Rel(basePath, filePath)
		if err != nil {
			return nil, err
		}
		newFilePath := filepath.Join(newFileNameRoot, relativePath)
		doc.Path = filepath.ToSlash(newFilePath)
		doc.Folder = filepath.Dir(newFilePath)
	} else {
		documentPath := filepath.ToSlash(serverConfig.DocumentPath + "/" + serverConfig.NewDocumentFolderRel + "/" + filepath.Base(filePath))
		doc.Path = documentPath
		documentFolder := filepath.ToSlash(serverConfig.DocumentPath + "/" + serverConfig.NewDocumentFolderRel)
		doc.Folder = documentFolder
	}

	// Save initial document record
	if err := db.SaveDocument(doc); err != nil {
		return nil, fmt.Errorf("unable to save document: %w", err)
	}

	return doc, nil
}

// moveAndVerifyFile moves the file to the documents folder and verifies the hash matches
func (serverHandler *ServerHandler) moveAndVerifyFile(sourcePath, destPath, expectedHash string) error {
	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source file
	srcFile, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(destPath, srcFile, os.ModePerm); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	// Verify hash of destination file
	destHash, err := calculateFileHash(destPath)
	if err != nil {
		// Cleanup: remove the copied file
		os.Remove(destPath)
		return fmt.Errorf("failed to verify destination file: %w", err)
	}

	if destHash != expectedHash {
		// Cleanup: remove the corrupted file
		os.Remove(destPath)
		return fmt.Errorf("hash mismatch after copy (expected: %s, got: %s)", expectedHash, destHash)
	}

	// Hash verified - now delete the source file
	if err := os.Remove(sourcePath); err != nil {
		Logger.Warn("Failed to delete source file after successful copy", "sourcePath", sourcePath, "error", err)
		// Don't fail the operation - the file was copied successfully
	}

	// If sidecar .txt file exists in ingress, move it to documents too
	if serverHandler.ServerConfig.UseSidecarTxt {
		sourceSidecarPath := getSidecarTxtPath(sourcePath)
		if _, err := os.Stat(sourceSidecarPath); err == nil {
			// Sidecar exists in ingress, move it to documents
			destSidecarPath := getSidecarTxtPath(destPath)
			sidecarContent, err := os.ReadFile(sourceSidecarPath)
			if err == nil {
				if err := os.WriteFile(destSidecarPath, sidecarContent, 0644); err != nil {
					Logger.Warn("Failed to copy sidecar .txt file", "source", sourceSidecarPath, "dest", destSidecarPath, "error", err)
				} else {
					Logger.Info("Moved sidecar .txt file", "source", sourceSidecarPath, "dest", destSidecarPath)
					// Delete source sidecar file
					os.Remove(sourceSidecarPath)
				}
			}
		}
	}

	return nil
}

// extractText extracts text from the document based on file type
func (serverHandler *ServerHandler) extractText(filePath string) (string, error) {
	// Check for sidecar .txt file first if enabled
	if serverHandler.ServerConfig.UseSidecarTxt {
		sidecarPath := getSidecarTxtPath(filePath)
		if content, err := os.ReadFile(sidecarPath); err == nil {
			Logger.Info("Using sidecar .txt file for text content", "document", filePath, "sidecar", sidecarPath)
			return string(content), nil
		}
		// Sidecar file doesn't exist, proceed with normal extraction
	}

	switch filepath.Ext(filePath) {
	case ".pdf":
		// Try direct PDF text extraction first
		fullText, err := pdfProcessing(filePath)
		if err != nil || fullText == nil || *fullText == "" {
			// Fallback to OCR
			fullText, err = serverHandler.convertToImage(filePath)
			if err != nil {
				return "", fmt.Errorf("OCR processing failed: %w", err)
			}
			if fullText == nil {
				return "", fmt.Errorf("PDF processing returned nil text")
			}
			return *fullText, nil
		}
		return *fullText, nil

	case ".tiff", ".jpg", ".jpeg", ".png":
		fullText, err := serverHandler.ocrProcessing(filePath)
		if err != nil {
			return "", fmt.Errorf("OCR processing failed: %w", err)
		}
		if fullText == nil {
			return "", fmt.Errorf("OCR processing returned nil text")
		}
		return *fullText, nil

	case ".txt", ".rtf":
		// For text files, read content directly
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read text file: %w", err)
		}
		return string(content), nil

	case ".doc", ".docx", ".odf":
		// These are not currently supported for text extraction
		return "", fmt.Errorf("text extraction not supported for %s files", filepath.Ext(filePath))

	default:
		return "", fmt.Errorf("unsupported file type: %s", filepath.Ext(filePath))
	}
}

// updateDocumentText updates the document with extracted text
func (serverHandler *ServerHandler) updateDocumentText(doc *database.Document, fullText string, db database.Repository) error {
	// Update the document's full text field
	doc.FullText = fullText

	// Save the updated document (SaveDocument does an upsert)
	err := db.SaveDocument(doc)
	if err != nil {
		return fmt.Errorf("unable to update full text: %w", err)
	}

	// Save sidecar .txt file if enabled and we have text
	if serverHandler.ServerConfig.UseSidecarTxt && fullText != "" {
		if err := saveSidecarTxtFile(doc.Path, fullText); err != nil {
			Logger.Warn("Failed to save sidecar .txt file", "document", doc.Path, "error", err)
			// Don't fail the ingestion if sidecar save fails
		}
	}

	// Generate thumbnail for supported document types
	if thumbnailSupported(doc.Path) {
		if err := saveThumbnailFile(doc.Path); err != nil {
			Logger.Warn("Failed to generate thumbnail", "document", doc.Path, "error", err)
			// Don't fail the ingestion if thumbnail generation fails
		}
	}

	// Read and apply tags and dimensions from sidecar file if it exists
	tagData, err := readTagsSidecar(doc.Path)
	if err != nil {
		Logger.Warn("Failed to read tags sidecar", "document", doc.Path, "error", err)
		// Don't fail ingestion if tags can't be read
	} else if tagData != nil {
		if err := serverHandler.applyTagsAndDimensionsToDocument(doc, tagData, db); err != nil {
			Logger.Warn("Failed to apply tags/dimensions to document", "document", doc.Path, "error", err)
			// Don't fail ingestion if tags can't be applied
		}
	}

	// PostgreSQL full-text search trigger will automatically update the search index
	return nil
}

// getSidecarTxtPath returns the path to the sidecar .txt file for a document
func getSidecarTxtPath(docPath string) string {
	ext := filepath.Ext(docPath)
	return docPath[:len(docPath)-len(ext)] + ".txt"
}

// saveSidecarTxtFile saves the extracted text to a .txt file alongside the document
func saveSidecarTxtFile(docPath, text string) error {
	sidecarPath := getSidecarTxtPath(docPath)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(sidecarPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for sidecar file: %w", err)
	}

	// Write text to file
	if err := os.WriteFile(sidecarPath, []byte(text), 0644); err != nil {
		return fmt.Errorf("failed to write sidecar file: %w", err)
	}

	Logger.Info("Saved sidecar .txt file", "document", docPath, "sidecar", sidecarPath, "textLength", len(text))
	return nil
}

// RescanOrphanedDocument imports a file that exists on disk but not in the database.
// Unlike regular ingestion, this does NOT move the file - it imports it in-place.
// Returns the file's hash for duplicate tracking.
func (serverHandler *ServerHandler) RescanOrphanedDocument(filePath string, db database.Repository, seenHashes map[string]string) (string, error) {
	fileName := filepath.Base(filePath)

	// Step 1: Calculate hash
	Logger.Info("Rescan: Calculating hash", "filePath", filePath)
	fileHash, err := calculateFileHash(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Step 2: Check for duplicate by hash in database
	existingDoc, err := db.GetDocumentByHash(fileHash)
	if err == nil && existingDoc != nil {
		Logger.Info("Rescan: Duplicate found in database, skipping", "filePath", filePath, "existingDoc", existingDoc.Path)
		return fileHash, fmt.Errorf("duplicate: already in database as %s", existingDoc.Path)
	}

	// Step 3: Check for duplicate by hash in current rescan batch (first occurrence wins)
	if existingPath, exists := seenHashes[fileHash]; exists {
		Logger.Info("Rescan: Duplicate found in scan batch, skipping", "filePath", filePath, "firstOccurrence", existingPath)
		return fileHash, fmt.Errorf("duplicate: same content as %s", existingPath)
	}

	// Step 4: Create database record with current path (no moving)
	newTime := time.Now()
	newULID, err := database.CalculateUUID(newTime)
	if err != nil {
		return fileHash, fmt.Errorf("cannot generate ULID: %w", err)
	}

	doc := &database.Document{
		Name:         fileName,
		Path:         filePath,
		Folder:       filepath.Dir(filePath),
		Hash:         fileHash,
		IngressTime:  newTime,
		ULID:         newULID,
		DocumentType: filepath.Ext(filePath),
		FullText:     "", // Will be populated below
	}

	// Save initial document record
	if err := db.SaveDocument(doc); err != nil {
		return fileHash, fmt.Errorf("failed to save document: %w", err)
	}
	Logger.Info("Rescan: Created database record", "ulid", doc.ULID.String(), "path", filePath)

	// Step 5: Extract text
	fullText, err := serverHandler.extractText(filePath)
	if err != nil {
		Logger.Warn("Rescan: Text extraction failed, storing document without text", "error", err, "fileName", fileName)
		fullText = ""
	}

	// Step 6: Update document with text and apply tags/dimensions/thumbnail
	if err := serverHandler.updateDocumentText(doc, fullText, db); err != nil {
		Logger.Error("Rescan: Failed to update document text", "error", err, "ulid", doc.ULID.String())
	}

	// Step 7: Set document URL
	documentURL := "/document/view/" + doc.ULID.String()
	if serverHandler.Echo != nil {
		serverHandler.Echo.File(documentURL, doc.Path)
	}
	if _, err := database.UpdateDocumentField(doc.ULID.String(), "URL", documentURL, db); err != nil {
		Logger.Error("Rescan: Failed to update document URL", "error", err, "ulid", doc.ULID.String())
	}

	Logger.Info("Rescan: Document imported successfully", "fileName", fileName, "ulid", doc.ULID.String(), "textLength", len(fullText))
	return fileHash, nil
}

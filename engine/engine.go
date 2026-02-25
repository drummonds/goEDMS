package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/drummonds/godocs/config"
	"github.com/drummonds/godocs/database"
	"github.com/drummonds/godocs/engine/pdfrenderer"
	"github.com/ledongthuc/pdf"
	"github.com/oklog/ulid/v2"
)

// shouldSkipFileForIngestion checks if a file should be skipped during ingestion
// Returns true for auxiliary/sidecar files (thumbnails, OCR text, tags metadata)
func shouldSkipFileForIngestion(filePath string) bool {
	fileName := filepath.Base(filePath)

	// Skip thumbnail files (legacy *.tn_*.png and canonical *.thumb.png)
	if isThumbnailFile(filePath) {
		return true
	}

	// Skip canonical sidecar files: *.ocr.txt, *.tags.json
	if strings.HasSuffix(fileName, ".ocr.txt") {
		return true
	}
	if strings.HasSuffix(fileName, ".tags.json") {
		return true
	}

	// Skip legacy sidecar .txt files if a corresponding main document exists
	if strings.HasSuffix(fileName, ".txt") {
		baseName := fileName[:len(fileName)-4] // Remove .txt extension
		dir := filepath.Dir(filePath)

		commonExts := []string{".pdf", ".jpg", ".jpeg", ".png", ".tiff"}
		for _, ext := range commonExts {
			possibleDoc := filepath.Join(dir, baseName+ext)
			if _, err := os.Stat(possibleDoc); err == nil {
				return true
			}
		}
	}

	return false
}

func (serverHandler *ServerHandler) ingressJobFunc(ctx context.Context, serverConfig config.ServerConfig, db database.Repository) {
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered in ingress job", "panic", r)
		}
	}()

	serverConfig, err := database.FetchConfigFromDB(db)
	if err != nil {
		Logger.Error("Error reading config from database", "error", err)
	}
	Logger.Info("Starting Ingress Job on folder", "path", serverConfig.IngressPath)
	var ingressPath []string
	err = filepath.Walk(serverConfig.IngressPath, func(path string, info os.FileInfo, err error) error {
		ingressPath = append(ingressPath, path)
		return nil
	})
	if err != nil {
		Logger.Error("Error reading files in from ingress", "error", err)
	}
	for _, filePath := range ingressPath {
		if ctx.Err() != nil {
			Logger.Info("Cron ingress job cancelled")
			return
		}
		Logger.Debug("Starting processing for file", "filePath", filePath)
		fileStats, err := os.Stat(filePath)
		if err != nil {
			Logger.Warn("Unable to get information for file, won't process", "filePath", filePath, "error", err)
			continue
		}
		if fileStats.IsDir() {
			Logger.Info("Skipping Folder", "filePath", filePath)
			continue
		}
		if filePath == serverConfig.IngressPath {
			Logger.Info("Skipping ingress Folder", "filePath", filePath)
			continue
		}
		if shouldSkipFileForIngestion(filePath) {
			Logger.Debug("Skipping auxiliary file", "filePath", filePath)
			continue
		}
		serverHandler.ingressDocument(filePath, "ingress")
	}
	deleteEmptyIngressFolders(serverHandler.ServerConfig.IngressPath)
}

func (serverHandler *ServerHandler) ingressJobFuncWithTracking(ctx context.Context, serverConfig config.ServerConfig, db database.Repository, jobID ulid.ULID) {
	defer serverHandler.unregisterJob(jobID)
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered in ingress job", "panic", r, "jobID", jobID)
			db.UpdateJobError(jobID, fmt.Sprintf("Panic: %v", r))
		}
	}()

	if err := db.UpdateJobStatus(jobID, database.JobStatusRunning, "Scanning ingress folder"); err != nil {
		Logger.Error("Failed to update job status", "error", err)
	}

	serverConfig, err := database.FetchConfigFromDB(db)
	if err != nil {
		Logger.Error("Error reading config from database", "error", err)
		db.UpdateJobError(jobID, fmt.Sprintf("Failed to fetch config: %v", err))
		return
	}

	Logger.Info("Starting Ingress Job with tracking", "path", serverConfig.IngressPath, "jobID", jobID)

	var ingressFiles []string
	err = filepath.Walk(serverConfig.IngressPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && path != serverConfig.IngressPath && !shouldSkipFileForIngestion(path) {
			ingressFiles = append(ingressFiles, path)
		}
		return nil
	})

	if err != nil {
		Logger.Error("Error scanning ingress folder", "error", err)
		db.UpdateJobError(jobID, fmt.Sprintf("Scan failed: %v", err))
		return
	}

	totalFiles := len(ingressFiles)
	if totalFiles == 0 {
		Logger.Info("No files to process in ingress folder")
		db.CompleteJob(jobID, `{"filesProcessed": 0, "message": "No files found"}`)
		return
	}

	Logger.Info("Found files to process", "count", totalFiles)
	processedFiles := 0
	errorCount := 0
	duplicateCount := 0

	for i, filePath := range ingressFiles {
		if ctx.Err() != nil {
			Logger.Info("Ingestion job cancelled", "jobID", jobID, "processed", i, "total", totalFiles)
			db.UpdateJobStatus(jobID, database.JobStatusCancelled,
				fmt.Sprintf("Cancelled after processing %d/%d files", i, totalFiles))
			return
		}

		fileName := filepath.Base(filePath)
		Logger.Info("Processing file with step-based ingestion", "file", fileName, "number", i+1, "total", totalFiles)

		err := serverHandler.IngestDocumentWithSteps(ctx, filePath, db, jobID, i, totalFiles)
		if err != nil {
			if strings.HasPrefix(err.Error(), "duplicate") {
				Logger.Info("Skipped duplicate document", "filePath", filePath)
				duplicateCount++
				processedFiles++
			} else {
				Logger.Error("Failed to process document", "filePath", filePath, "error", err)
				errorCount++
			}
		} else {
			processedFiles++
		}
	}

	deleteEmptyIngressFolders(serverConfig.IngressPath)

	db.UpdateJobProgress(jobID, 95, "Updating word cloud")
	if err := db.RecalculateAllWordFrequencies(); err != nil {
		Logger.Error("Word cloud recalculation failed after ingestion", "error", err)
	}

	result := fmt.Sprintf(`{"filesProcessed": %d, "filesTotal": %d, "errors": %d, "duplicates": %d}`, processedFiles, totalFiles, errorCount, duplicateCount)
	if err := db.CompleteJob(jobID, result); err != nil {
		Logger.Error("Failed to mark job as complete", "error", err)
	}

	Logger.Info("Ingestion job completed", "jobID", jobID, "processed", processedFiles, "total", totalFiles, "errors", errorCount, "duplicates", duplicateCount)
}

func (serverHandler *ServerHandler) cleanupJobFuncWithTracking(ctx context.Context, db database.Repository, jobID ulid.ULID) {
	defer serverHandler.unregisterJob(jobID)
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered in cleanup job", "panic", r, "jobID", jobID)
			db.UpdateJobError(jobID, fmt.Sprintf("Panic: %v", r))
		}
	}()

	cancelled := func() bool {
		if ctx.Err() != nil {
			db.UpdateJobStatus(jobID, database.JobStatusCancelled, "Cancelled")
			return true
		}
		return false
	}

	db.UpdateJobStatus(jobID, database.JobStatusRunning, "Fetching documents from database")

	documentsPtr, err := database.FetchAllDocuments(db)
	if err != nil {
		Logger.Error("Failed to fetch documents for cleanup", "error", err)
		db.UpdateJobError(jobID, fmt.Sprintf("Failed to fetch documents: %v", err))
		return
	}

	if documentsPtr == nil {
		db.CompleteJob(jobID, `{"scanned": 0, "deleted": 0, "moved": 0}`)
		return
	}

	documents := *documentsPtr
	totalDocs := len(documents)
	deletedCount := 0

	Logger.Info("Starting database cleanup", "total_documents", totalDocs)
	db.UpdateJobProgress(jobID, 10, fmt.Sprintf("Checking %d documents", totalDocs))

	sidecarTxtRemoved := 0
	for i, doc := range documents {
		if cancelled() {
			return
		}
		if doc.Path == "" {
			continue
		}

		progress := 10 + int((float64(i)/float64(totalDocs))*40)
		db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Checking document %d/%d", i+1, totalDocs))

		if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
			Logger.Info("File not found, removing from database", "path", doc.Path, "id", doc.ID)
			if err := database.DeleteDocument(doc.ULID.String(), db); err != nil {
				Logger.Error("Failed to delete document from DB", "error", err, "id", doc.ID)
				continue
			}
			deletedCount++
			continue
		}

		if strings.ToLower(filepath.Ext(doc.Path)) == ".txt" {
			if !isTxtRootDocument(doc.Path) {
				Logger.Info("Removing .txt sidecar entry from database", "path", doc.Path, "id", doc.ID)
				if err := database.DeleteDocument(doc.ULID.String(), db); err != nil {
					Logger.Error("Failed to delete .txt sidecar from DB", "error", err, "id", doc.ID)
					continue
				}
				sidecarTxtRemoved++
			}
		}
	}

	tempPurged := 0
	for _, doc := range documents {
		if strings.HasPrefix(doc.Path, "__temp__/") || strings.HasPrefix(doc.Folder, "__temp__/") {
			Logger.Info("Purging incomplete ingestion entry", "path", doc.Path, "id", doc.ID)
			if err := database.DeleteDocument(doc.ULID.String(), db); err != nil {
				Logger.Error("Failed to purge temp entry", "error", err, "id", doc.ID)
			} else {
				tempPurged++
			}
		}
	}
	if tempPurged > 0 {
		Logger.Info("Purged incomplete ingestion entries", "count", tempPurged)
	}

	if cancelled() {
		return
	}

	db.UpdateJobProgress(jobID, 55, "Restoring tag aliases from config")
	if err := ApplyTagAliasesFromConfig(serverHandler.ServerConfig.ConfigPath, db); err != nil {
		Logger.Error("Failed to apply tag aliases from config", "error", err)
	}

	db.UpdateJobProgress(jobID, 60, "Scanning for orphaned files")
	rescannedCount := 0
	duplicateCount := 0
	seenHashes := make(map[string]string)

	for _, doc := range documents {
		if doc.Hash != "" {
			seenHashes[doc.Hash] = doc.Path
		}
	}

	orphanedFiles, err := serverHandler.findOrphanedDocuments(documents)
	if err != nil {
		Logger.Error("Failed to scan for orphaned documents", "error", err)
	} else {
		totalOrphans := len(orphanedFiles)
		Logger.Info("Found orphaned files to rescan", "count", totalOrphans)
		for i, orphanPath := range orphanedFiles {
			if cancelled() {
				return
			}
			progress := 60 + int((float64(i)/float64(totalOrphans))*20)
			db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Rescanning orphan %d/%d", i+1, totalOrphans))

			fileHash, err := serverHandler.RescanOrphanedDocument(orphanPath, db, seenHashes)
			if err != nil {
				if fileHash != "" {
					duplicateCount++
					Logger.Info("Skipped duplicate file during rescan", "path", orphanPath, "reason", err.Error())
				} else {
					Logger.Error("Failed to rescan orphaned document", "path", orphanPath, "error", err)
				}
			} else {
				seenHashes[fileHash] = orphanPath
				rescannedCount++
			}
		}
	}

	if cancelled() {
		return
	}

	db.UpdateJobProgress(jobID, 70, "Checking sidecar .txt files")
	sidecarCount := 0
	if serverHandler.ServerConfig.UseSidecarTxt {
		Logger.Info("Checking for missing sidecar .txt files")
		lastSidecarUpdate := time.Now()
		for i, doc := range documents {
			if doc.Path == "" {
				continue
			}
			if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
				continue
			}
			sidecarPath := getOCRPath(doc.Path)
			if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
				if doc.FullText != "" {
					Logger.Info("Recreating missing sidecar .txt file", "document", doc.Path, "sidecar", sidecarPath)
					if err := saveOCRFile(doc.Path, doc.FullText); err != nil {
						Logger.Error("Failed to recreate sidecar .txt file", "document", doc.Path, "error", err)
					} else {
						sidecarCount++
					}
				}
			}
			if time.Since(lastSidecarUpdate) >= 5*time.Second || i == totalDocs-1 {
				progress := 70 + int((float64(i+1)/float64(totalDocs))*5)
				db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Checking sidecar files %d/%d", i+1, totalDocs))
				lastSidecarUpdate = time.Now()
			}
		}
		Logger.Info("Sidecar .txt file check complete", "recreated", sidecarCount)
	}

	if cancelled() {
		return
	}

	db.UpdateJobProgress(jobID, 75, "Checking document thumbnails")
	thumbnailCount := 0
	thumbnailsChecked := 0
	Logger.Info("Checking for missing thumbnails")
	lastThumbnailUpdate := time.Now()
	for i, doc := range documents {
		if doc.Path == "" || !thumbnailSupported(doc.Path) {
			continue
		}
		if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
			continue
		}
		thumbnailsChecked++
		thumbnailPath := getThumbPath(doc.Path)
		if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
			Logger.Info("Generating missing thumbnail", "document", doc.Path, "thumbnail", thumbnailPath)
			if err := saveThumbnailFile(doc.Path); err != nil {
				Logger.Error("Failed to generate thumbnail", "document", doc.Path, "error", err)
			} else {
				thumbnailCount++
			}
		}
		if time.Since(lastThumbnailUpdate) >= 5*time.Second || i == totalDocs-1 {
			progress := 75 + int((float64(i+1)/float64(totalDocs))*10)
			db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Checking thumbnails %d/%d (generated %d)", i+1, totalDocs, thumbnailCount))
			lastThumbnailUpdate = time.Now()
		}
	}
	Logger.Info("Thumbnail check complete", "checked", thumbnailsChecked, "generated", thumbnailCount)

	db.UpdateJobProgress(jobID, 80, "Cleaning orphaned sidecar files")
	orphanedSidecarsDeleted := serverHandler.cleanOrphanedSidecars()
	Logger.Info("Orphaned sidecar cleanup complete", "deleted", orphanedSidecarsDeleted)

	if cancelled() {
		return
	}

	db.UpdateJobProgress(jobID, 83, "Migrating documents to canonical naming")
	nestedMigratedCount := 0
	Logger.Info("Checking for documents needing canonical naming migration")
	freshDocsPtr, err := database.FetchAllDocuments(db)
	if err == nil && freshDocsPtr != nil {
		documentRoot := serverHandler.ServerConfig.DocumentPath
		for _, doc := range *freshDocsPtr {
			expectedPath, expectedFolder := ComputeNestedPath(doc.ID, doc.DocumentType, documentRoot)
			if doc.Path == expectedPath {
				continue
			}
			if _, statErr := os.Stat(doc.Path); statErr != nil {
				continue
			}
			if err := migrateToCanonicalNaming(doc, expectedPath); err != nil {
				Logger.Error("Failed to migrate document to canonical naming", "id", doc.ID, "from", doc.Path, "to", expectedPath, "error", err)
				continue
			}
			if err := db.UpdateDocumentPath(doc.ULID.String(), expectedPath, expectedFolder); err != nil {
				Logger.Error("Failed to update DB after canonical migration", "id", doc.ID, "error", err)
				continue
			}
			nestedMigratedCount++
			Logger.Info("Migrated document to canonical naming", "id", doc.ID, "from", doc.Path, "to", expectedPath)
		}
	}
	Logger.Info("Canonical naming migration complete", "migrated", nestedMigratedCount)

	db.UpdateJobProgress(jobID, 87, "Migrating legacy JSON fields")
	jsonMigratedCount := serverHandler.migrateStormIDInTagsFiles()
	Logger.Info("JSON field migration complete", "migrated", jsonMigratedCount)

	db.UpdateJobProgress(jobID, 90, "Recalculating word cloud")
	if err := db.RecalculateAllWordFrequencies(); err != nil {
		Logger.Error("Word cloud recalculation failed after cleanup", "error", err)
	}

	db.UpdateJobProgress(jobID, 93, "Cleaning ingress folder")
	ingressCleaned := 0
	ingressPath := serverHandler.ServerConfig.IngressPath
	if ingressPath != "" {
		ingressCleaned = serverHandler.cleanIngressFolder(db)
		deleteEmptyIngressFolders(ingressPath)
	}
	Logger.Info("Ingress cleanup complete", "deleted", ingressCleaned)

	result := fmt.Sprintf(`{"scanned": %d, "deleted": %d, "tempPurged": %d, "sidecarTxtRemoved": %d, "rescanned": %d, "duplicatesSkipped": %d, "sidecarRecreated": %d, "thumbnailsChecked": %d, "thumbnailsGenerated": %d, "orphanedSidecarsDeleted": %d, "nestedMigrated": %d, "jsonFilesMigrated": %d, "ingressCleaned": %d}`, totalDocs, deletedCount, tempPurged, sidecarTxtRemoved, rescannedCount, duplicateCount, sidecarCount, thumbnailsChecked, thumbnailCount, orphanedSidecarsDeleted, nestedMigratedCount, jsonMigratedCount, ingressCleaned)
	if err := db.CompleteJob(jobID, result); err != nil {
		Logger.Error("Failed to mark cleanup job as complete", "error", err)
	}

	Logger.Info("Database cleanup job completed", "jobID", jobID, "scanned", totalDocs, "deleted", deletedCount, "tempPurged", tempPurged, "sidecarTxtRemoved", sidecarTxtRemoved, "rescanned", rescannedCount, "duplicatesSkipped", duplicateCount, "orphanedSidecarsDeleted", orphanedSidecarsDeleted, "nestedMigrated", nestedMigratedCount, "jsonFilesMigrated", jsonMigratedCount, "ingressCleaned", ingressCleaned)
}

// migrateToCanonicalNaming moves a document file to its canonical path and renames
// legacy sidecars to canonical names. Uses rename with copy+delete fallback.
func migrateToCanonicalNaming(doc database.Document, newDocPath string) error {
	if err := os.MkdirAll(filepath.Dir(newDocPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Move main file
	if err := renameWithFallback(doc.Path, newDocPath); err != nil {
		return fmt.Errorf("failed to move main file: %w", err)
	}

	// Compute old base path (strip extension from old path)
	oldExt := filepath.Ext(doc.Path)
	oldBase := doc.Path[:len(doc.Path)-len(oldExt)]

	// Compute new sidecar base from canonical path
	newBase := SidecarBasePath(newDocPath)

	// Compute old canonical sidecar base (for L/K/J → A-Z migration)
	oldCanonicalBase := SidecarBasePath(doc.Path)

	// Sidecar mappings: legacy names first, then canonical names (for tier migration)
	sidecarMappings := []struct {
		oldPath string
		newPath string
	}{
		// Legacy sidecar names
		{oldBase + ".txt", newBase + ".ocr.txt"},
		{oldBase + ".tn_256.png", newBase + ".thumb.png"},
		{oldBase + ".tags.json", newBase + ".tags.json"},
		// Canonical sidecar names (L/K/J → A-Z migration)
		{oldCanonicalBase + ".ocr.txt", newBase + ".ocr.txt"},
		{oldCanonicalBase + ".thumb.png", newBase + ".thumb.png"},
		{oldCanonicalBase + ".tags.json", newBase + ".tags.json"},
	}

	for _, m := range sidecarMappings {
		if _, err := os.Stat(m.oldPath); err == nil {
			if err := renameWithFallback(m.oldPath, m.newPath); err != nil {
				Logger.Warn("Failed to migrate sidecar", "from", m.oldPath, "to", m.newPath, "error", err)
			}
		}
	}

	return nil
}

// renameWithFallback tries os.Rename, falling back to copy+delete for cross-device moves.
func renameWithFallback(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		data, err2 := os.ReadFile(src)
		if err2 != nil {
			return fmt.Errorf("failed to read source: %w", err2)
		}
		if err2 := os.WriteFile(dst, data, os.ModePerm); err2 != nil {
			return fmt.Errorf("failed to write destination: %w", err2)
		}
		os.Remove(src)
	}
	return nil
}

// cleanIngressFolder walks the ingress folder and deletes files that have already
// been ingested (verified by matching file hash against the database).
// Also deletes sidecar files (.txt, .tags.json) whose companion document has been ingested.
// Returns the number of files deleted.
func (serverHandler *ServerHandler) cleanIngressFolder(db database.Repository) int {
	ingressPath := serverHandler.ServerConfig.IngressPath
	deletedCount := 0

	err := filepath.Walk(ingressPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip sidecar/auxiliary files on first pass — they'll be cleaned
		// if their companion document is gone
		if shouldSkipFileForIngestion(path) {
			return nil
		}

		// Calculate hash and check if already in DB
		fileHash, err := CalculateFileHash(path)
		if err != nil {
			Logger.Warn("Failed to hash ingress file", "path", path, "error", err)
			return nil
		}

		doc, err := db.GetDocumentByHash(fileHash)
		if err != nil || doc == nil {
			// Not in DB — leave it for future ingestion
			return nil
		}

		// Verify the ingested file actually exists on disk
		if _, statErr := os.Stat(doc.Path); statErr != nil {
			Logger.Info("Ingress file matches DB hash but document file missing, leaving for re-ingestion",
				"ingress", path, "dbPath", doc.Path)
			return nil
		}

		// Already ingested — safe to delete from ingress
		Logger.Info("Deleting already-ingested file from ingress", "path", path, "hash", fileHash, "dbDoc", doc.Path)
		if err := os.Remove(path); err != nil {
			Logger.Error("Failed to delete ingress file", "path", path, "error", err)
		} else {
			deletedCount++
		}

		// Also delete companion sidecar files in ingress
		ext := filepath.Ext(path)
		basePath := path[:len(path)-len(ext)]
		for _, suffix := range []string{".txt", ".tags.json"} {
			sidecarPath := basePath + suffix
			if _, err := os.Stat(sidecarPath); err == nil {
				Logger.Info("Deleting ingress sidecar for ingested document", "path", sidecarPath)
				if err := os.Remove(sidecarPath); err != nil {
					Logger.Error("Failed to delete ingress sidecar", "path", sidecarPath, "error", err)
				} else {
					deletedCount++
				}
			}
		}

		return nil
	})

	if err != nil {
		Logger.Error("Error walking ingress folder for cleanup", "error", err)
	}

	return deletedCount
}

// ingressDocumentWithError is like ingressDocument but returns errors instead of just logging
func (serverHandler *ServerHandler) ingressDocumentWithError(filePath string, source string) error {
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered while processing document", "filePath", filePath, "panic", r)
		}
	}()

	switch filepath.Ext(filePath) {
	case ".pdf":
		fullText, err := pdfProcessing(filePath)
		if err != nil {
			fullText, err = serverHandler.convertToImage(filePath)
			if err != nil {
				return fmt.Errorf("OCR processing failed: %w", err)
			}
		}
		if fullText == nil {
			return fmt.Errorf("PDF processing returned nil text")
		}
		return serverHandler.addDocumentToDatabase(filePath, *fullText, source)

	case ".txt", ".rtf":
		textProcessing(filePath)
		return nil

	case ".doc", ".docx", ".odf":
		wordDocProcessing(filePath)
		return nil

	case ".tiff", ".jpg", ".jpeg", ".png":
		fullText, err := serverHandler.ocrProcessing(filePath)
		if err != nil {
			return fmt.Errorf("OCR processing failed: %w", err)
		}
		if fullText == nil {
			return fmt.Errorf("OCR processing returned nil text")
		}
		return serverHandler.addDocumentToDatabase(filePath, *fullText, source)

	default:
		return fmt.Errorf("unsupported file type: %s", filepath.Ext(filePath))
	}
}

func (serverHandler *ServerHandler) ingressDocument(filePath string, source string) { //source is either from ingress folder or from upload
	// Add panic recovery to prevent one bad document from crashing the entire ingress job
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered while processing document", "filePath", filePath, "panic", r)
		}
	}()

	switch filepath.Ext(filePath) {
	case ".pdf":
		fullText, err := pdfProcessing(filePath)
		if err != nil {
			fullText, err = serverHandler.convertToImage(filePath)
			if err != nil {
				Logger.Error("OCR Processing failed on file so not added to database", "filePath", filePath, "error", err)
				return
			}
		}
		// Check if fullText is nil before dereferencing
		if fullText == nil {
			Logger.Error("PDF processing returned nil text, skipping document", "filePath", filePath)
			return
		}
		serverHandler.addDocumentToDatabase(filePath, *fullText, source)

	case ".txt", ".rtf":
		textProcessing(filePath)
	case ".doc", ".docx", ".odf":
		wordDocProcessing(filePath)
	case ".tiff", ".jpg", ".jpeg", ".png":
		fullText, err := serverHandler.ocrProcessing(filePath)
		if err != nil {
			Logger.Error("OCR Processing failed on file", "filePath", filePath, "error", err)
			return
		}
		// Check if fullText is nil before dereferencing
		if fullText == nil {
			Logger.Error("OCR processing returned nil text, skipping document", "filePath", filePath)
			return
		}
		serverHandler.addDocumentToDatabase(filePath, *fullText, source)
	default:
		Logger.Warn("Invalid file type", "file", filepath.Base((filePath)))
	}
}

func (serverHandler *ServerHandler) addDocumentToDatabase(filePath string, fullText string, source string) error {
	document, err := database.AddNewDocument(filePath, fullText, serverHandler.DB) //Adds everything but the URL, that is added afterwards
	if err != nil {
		Logger.Error("Failed to add document to database", "document", document, "error", err) //TODO: Handle document that we were unable to add
		return err
	}
	documentURL := "/document/view/" + document.ULID.String()
	serverHandler.Echo.File(documentURL, document.Path)                                                 //Generating a direct URL to document so it is live immediately after add
	_, err = database.UpdateDocumentField(document.ULID.String(), "URL", documentURL, serverHandler.DB) //updating the database with the new file location
	if err != nil {
		Logger.Error("Unable to update document field", "field", "Path", "error", err)
		return err
	}
	err = ingressCopyDocument(filePath, serverHandler.ServerConfig)
	if err != nil {
		Logger.Error("Error moving ingress file to new location", "filePath", filePath, "error", err)
		return err
	}
	if source == "ingress" { //if file was ingressed need to handle the original, if uploaded no problem
		err := ingressCleanup(filePath, *document, serverHandler.ServerConfig, serverHandler.DB)
		if err != nil {
			return err
		}
	}
	Logger.Info("Added file to the database", "filePath", filePath)
	return nil
}

func deleteEmptyIngressFolders(path string) {
	Logger.Info("Running cleanup on ingress folder", "path", path)
	err := filepath.Walk(path, func(currentFile string, info os.FileInfo, err error) error {
		f, err := os.Open(currentFile)
		if err != nil {
			return err
		}
		defer f.Close()
		Logger.Debug("Checking on path", "currentFile", currentFile)
		if path == currentFile {
			Logger.Debug("Skipping root dir", "path", path)
			return nil
		}

		_, err = f.Readdirnames(1)
		if err == io.EOF {
			Logger.Debug("Removing Empty Folder", "currentFile", currentFile)
			os.RemoveAll(currentFile)
			return nil
		}
		return nil
	})
	if err != nil {
		Logger.Error("Error cleaning ingress folder", "path", path, "error", err)
	}
}

// DeleteFile deletes a folder (or file) and everything in that folder
func DeleteFile(filePath string) error {
	err := os.RemoveAll(filePath)
	if err != nil {
		Logger.Error("Error deleting File/Folder", "error", err)
		return err
	}
	return nil
}

//DeleteDocumentFile deletes a file from the filesystem(database deletion handled in db)  //TODO Not sure if needed, might just use removeAll
/* func DeleteDocumentFile(filePath string) error {
	err := os.Remove(filePath)
	if err != nil {
		Logger.Error("Unable to delete file", "error", err)
		return err
	}
	return nil
} */

// ingressCopyDocument copies the document to document storage location
func ingressCopyDocument(filePath string, serverConfig config.ServerConfig) error {
	srcFile, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var newFilePath string
	if serverConfig.IngressPreserve == false { //if we are not saving the folder structure just read each file in with new path
		newFilePath = filepath.ToSlash(serverConfig.NewDocumentFolder + "/" + filepath.Base(filePath))
	} else { //If we ARE preserving ingress structure, create a new full path by creating a relative path and joining it to the
		basePath := serverConfig.IngressPath
		newFileNameRoot := serverConfig.DocumentPath
		relativePath, err := filepath.Rel(basePath, filePath)
		if err != nil {
			return err
		}
		newFilePath = filepath.Join(newFileNameRoot, relativePath)
		os.MkdirAll(filepath.Dir(newFilePath), os.ModePerm) //creating the directory structure so we can write the file: TODO: not sure if os.WriteFile does this for us?  Don't think so.
	}
	err = os.WriteFile(newFilePath, srcFile, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

// ingressCleanup cleans up the ingress folder after we have handled the documents //TODO: Maybe ALSO preserve folder structure from ingress folder here as well?
func ingressCleanup(fileName string, document database.Document, serverConfig config.ServerConfig, db database.Repository) error {
	if serverConfig.IngressDelete == true { //deleting the ingress files
		err := os.Remove(fileName)
		if err != nil {
			return err
		}
		return nil
	}
	newFile := filepath.FromSlash(serverConfig.IngressMoveFolder + "/" + filepath.Base(fileName)) //Moving ingress files to another location
	err := os.Rename(fileName, newFile)
	if err != nil {
		return err
	}
	return nil
}

func pdfProcessing(file string) (*string, error) {
	fileName := filepath.Base((file))
	var fullText string
	Logger.Debug("Working on current file", "fileName", fileName)
	pdfFile, result, err := pdf.Open(file)
	if err != nil {
		Logger.Error("Unable to open PDF", "fileName", fileName)
		return nil, err
	}
	defer pdfFile.Close()
	var buf bytes.Buffer
	bytes, err := result.GetPlainText()
	if err != nil {
		Logger.Error("Unable to convert PDF to text", "fileName", fileName)
		return nil, err
	}
	buf.ReadFrom(bytes)
	fullText = buf.String() //writing from the buffer to the string
	if fullText == "" {
		err = errors.New("PDF Text Result is empty")
		Logger.Info("PDF Text Result is empty, sending to OCR", "fileName", fileName, "error", err)
		return nil, err
	}
	Logger.Info("Text processed from PDF without OCR", "fileName", fileName)
	return &fullText, nil
}

func textProcessing(fileName string) {

}

func wordDocProcessing(fileName string) {

}

func (serverHandler *ServerHandler) convertToImage(fileName string) (*string, error) {
	var err error
	Logger.Info("Converting PDF To image for OCR using Go libraries", "fileName", fileName)

	// Create temporary image file for OCR
	baseName := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	tmpFile, err := os.CreateTemp("", baseName+"-*.png")
	if err != nil {
		Logger.Error("Unable to create temp file for OCR image", "fileName", fileName, "error", err)
		return nil, err
	}
	imageName := tmpFile.Name()
	tmpFile.Close() // Will be written to below
	defer os.Remove(imageName)

	fileName = filepath.Clean(fileName)
	Logger.Info("Creating temp image for OCR at", "imageName", imageName)

	// Check if file exists and is readable
	if _, err := os.Stat(fileName); err != nil {
		Logger.Error("Unable to access PDF file", "fileName", fileName, "error", err)
		return nil, err
	}

	// Create PDFium renderer (pure Go, no CGo)
	renderer, err := pdfrenderer.NewRenderer()
	if err != nil {
		Logger.Error("Unable to create PDF renderer (PDFium)", "error", err)
		return nil, err
	}
	defer renderer.Close()

	Logger.Debug("Using PDFium renderer (pure Go)")

	// Render all pages of the PDF to images
	images, err := renderer.RenderPDF(fileName)
	if err != nil {
		Logger.Error("Unable to render PDF pages", "fileName", fileName, "error", err)
		return nil, err
	}

	Logger.Debug("PDF has pages", "count", len(images))

	if len(images) == 0 {
		err := fmt.Errorf("no pages could be rendered from PDF")
		Logger.Error("Failed to render any pages", "fileName", fileName)
		return nil, err
	}

	// Combine all pages vertically (append)
	var combinedImage image.Image
	if len(images) == 1 {
		combinedImage = images[0]
	} else {
		// Calculate total height and max width
		totalHeight := 0
		maxWidth := 0
		for _, img := range images {
			bounds := img.Bounds()
			totalHeight += bounds.Dy()
			if bounds.Dx() > maxWidth {
				maxWidth = bounds.Dx()
			}
		}

		// Create combined image
		combined := image.NewRGBA(image.Rect(0, 0, maxWidth, totalHeight))
		currentY := 0
		for _, img := range images {
			bounds := img.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					combined.Set(x, currentY+y-bounds.Min.Y, img.At(x, y))
				}
			}
			currentY += bounds.Dy()
		}
		combinedImage = combined
	}

	// Resize to 1024px width while maintaining aspect ratio
	resizedImage := imaging.Resize(combinedImage, 1024, 0, imaging.Lanczos)

	// Apply basic sharpening to improve OCR quality
	processedImage := imaging.Sharpen(resizedImage, 1.0)

	// Save the processed image
	outFile, err := os.Create(imageName)
	if err != nil {
		Logger.Error("Unable to create output image file", "imageName", imageName, "error", err)
		return nil, err
	}
	defer outFile.Close()

	err = png.Encode(outFile, processedImage)
	if err != nil {
		Logger.Error("Unable to encode PNG image", "imageName", imageName, "error", err)
		return nil, err
	}

	Logger.Info("Successfully converted PDF to image", "imageName", imageName)

	fullText, err := serverHandler.ocrProcessing(imageName)
	if err != nil {
		return nil, err
	}
	return fullText, nil
}

func (serverHandler *ServerHandler) ocrProcessing(imageName string) (*string, error) {
	// Check if Tesseract is configured
	if serverHandler.ServerConfig.TesseractPath == "" {
		Logger.Info("Tesseract not configured, skipping OCR processing", "imageName", imageName)
		emptyText := ""
		return &emptyText, nil
	}

	// Tesseract takes an output base path and appends ".txt" itself
	baseName := strings.TrimSuffix(filepath.Base(imageName), filepath.Ext(imageName))
	tmpFile, err := os.CreateTemp("", baseName+"-ocr-*")
	if err != nil {
		Logger.Error("Unable to create temp file for OCR output", "error", err)
		return nil, err
	}
	textFileBase := tmpFile.Name()
	tmpFile.Close()
	os.Remove(textFileBase) // Tesseract creates its own file
	defer os.Remove(textFileBase + ".txt")

	tesseractArgs := []string{imageName, textFileBase}
	tesseractCMD := exec.Command(serverHandler.ServerConfig.TesseractPath, tesseractArgs...)
	var stdBuffer bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &stdBuffer)

	tesseractCMD.Stdout = mw
	tesseractCMD.Stderr = mw

	err = tesseractCMD.Run()
	Logger.Debug("Tesseract Command Run was", "command", tesseractCMD.String())
	if err != nil {
		Logger.Warn("Tesseract encountered error when attempting to OCR image, storing document without text", "imageName", imageName, "detail", stdBuffer.String())
		emptyText := ""
		return &emptyText, nil
	}
	fileBytes, err := os.ReadFile(textFileBase + ".txt")
	if err != nil {
		Logger.Warn("Unable to read OCR output file, storing document without text", "textFile", textFileBase+".txt", "error", err)
		emptyText := ""
		return &emptyText, nil
	}
	var fullText string
	fullText = string(fileBytes)
	if fullText == "" {
		Logger.Info("OCR returned empty string - document may have no recognizable text (e.g., handwritten, blank, or image-only)", "imageName", imageName)
		// Empty text is valid - return it successfully
	}
	return &fullText, nil
}

// migrateStormIDInTagsFiles scans all .tags.json files and migrates "stormid" fields to "id"
// This is a legacy cleanup for files created with the old Storm ORM
// Returns the number of files that were migrated
func (serverHandler *ServerHandler) migrateStormIDInTagsFiles() int {
	documentPath := serverHandler.ServerConfig.DocumentPath
	migratedCount := 0

	err := filepath.Walk(documentPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Only process .tags.json files
		if info.IsDir() || !strings.HasSuffix(path, ".tags.json") {
			return nil
		}

		// Read the file
		data, err := os.ReadFile(path)
		if err != nil {
			Logger.Warn("Failed to read tags file for migration", "path", path, "error", err)
			return nil
		}

		// Check if file contains "stormid" (case-insensitive check)
		content := string(data)
		if !strings.Contains(strings.ToLower(content), "stormid") {
			return nil // No migration needed
		}

		// Parse as generic JSON to preserve structure
		var jsonData map[string]interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			Logger.Warn("Failed to parse tags file for migration", "path", path, "error", err)
			return nil
		}

		// Migrate stormid to id at root level
		modified := false
		if val, exists := jsonData["stormid"]; exists {
			if _, hasID := jsonData["id"]; !hasID {
				jsonData["id"] = val
			}
			delete(jsonData, "stormid")
			modified = true
		}
		if val, exists := jsonData["StormID"]; exists {
			if _, hasID := jsonData["id"]; !hasID {
				jsonData["id"] = val
			}
			delete(jsonData, "StormID")
			modified = true
		}

		// Also check nested structures (e.g., arrays of objects)
		modified = migrateStormIDRecursive(jsonData) || modified

		if !modified {
			return nil
		}

		// Write back the migrated file
		newData, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			Logger.Warn("Failed to marshal migrated tags file", "path", path, "error", err)
			return nil
		}

		if err := os.WriteFile(path, newData, 0644); err != nil {
			Logger.Warn("Failed to write migrated tags file", "path", path, "error", err)
			return nil
		}

		Logger.Info("Migrated stormid to id in tags file", "path", path)
		migratedCount++
		return nil
	})

	if err != nil {
		Logger.Error("Error walking document path for stormid migration", "error", err)
	}

	return migratedCount
}

// migrateStormIDRecursive recursively migrates stormid fields in nested structures
func migrateStormIDRecursive(data interface{}) bool {
	modified := false

	switch v := data.(type) {
	case map[string]interface{}:
		// Migrate at this level
		if val, exists := v["stormid"]; exists {
			if _, hasID := v["id"]; !hasID {
				v["id"] = val
			}
			delete(v, "stormid")
			modified = true
		}
		if val, exists := v["StormID"]; exists {
			if _, hasID := v["id"]; !hasID {
				v["id"] = val
			}
			delete(v, "StormID")
			modified = true
		}
		// Recurse into nested values
		for _, val := range v {
			if migrateStormIDRecursive(val) {
				modified = true
			}
		}
	case []interface{}:
		// Recurse into array elements
		for _, item := range v {
			if migrateStormIDRecursive(item) {
				modified = true
			}
		}
	}

	return modified
}

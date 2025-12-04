package engine

import (
	"bytes"
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
// Returns true for auxiliary files like thumbnails (.tn_64.png) and sidecar text files
func shouldSkipFileForIngestion(filePath string) bool {
	fileName := filepath.Base(filePath)

	// Skip thumbnail files (e.g., document.tn_64.png)
	if strings.HasSuffix(fileName, ".tn_64.png") {
		return true
	}

	// Skip sidecar .txt files - these are handled separately during ingestion
	// Only skip if the corresponding main document exists
	if strings.HasSuffix(fileName, ".txt") {
		// Check if this is a sidecar file by looking for a corresponding document
		baseName := fileName[:len(fileName)-4] // Remove .txt extension
		dir := filepath.Dir(filePath)

		// Common document extensions that might have sidecar files
		commonExts := []string{".pdf", ".jpg", ".jpeg", ".png", ".tiff"}
		for _, ext := range commonExts {
			possibleDoc := filepath.Join(dir, baseName+ext)
			if _, err := os.Stat(possibleDoc); err == nil {
				// Found corresponding document, this is a sidecar file
				return true
			}
		}
	}

	return false
}

func (serverHandler *ServerHandler) ingressJobFunc(serverConfig config.ServerConfig, db database.Repository) {
	// Add panic recovery to prevent entire application crash
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
	deleteEmptyIngressFolders(serverHandler.ServerConfig.IngressPath) //after ingress clean empty folders
}

// ingressJobFuncWithTracking wraps the ingress job with progress tracking
func (serverHandler *ServerHandler) ingressJobFuncWithTracking(serverConfig config.ServerConfig, db database.Repository, jobID ulid.ULID) {
	// Add panic recovery and update job status on panic
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered in ingress job", "panic", r, "jobID", jobID)
			db.UpdateJobError(jobID, fmt.Sprintf("Panic: %v", r))
		}
	}()

	// Mark job as running
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

	// Scan for files
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
		db.CompleteJob(jobID, fmt.Sprintf(`{"filesProcessed": 0, "message": "No files found"}`))
		return
	}

	Logger.Info("Found files to process", "count", totalFiles)
	processedFiles := 0
	errorCount := 0
	duplicateCount := 0

	// Process each file with detailed step tracking
	for i, filePath := range ingressFiles {
		fileName := filepath.Base(filePath)

		Logger.Info("Processing file with step-based ingestion", "file", fileName, "number", i+1, "total", totalFiles)

		// Process the document using new step-based approach
		err := serverHandler.IngestDocumentWithSteps(filePath, db, jobID, i, totalFiles)
		if err != nil {
			if len(err.Error()) >= 9 && err.Error()[:9] == "duplicate" {
				Logger.Info("Skipped duplicate document", "filePath", filePath)
				duplicateCount++
				processedFiles++ // Count as processed (successfully skipped)
			} else {
				Logger.Error("Failed to process document", "filePath", filePath, "error", err)
				errorCount++
			}
		} else {
			processedFiles++
		}
	}

	// Clean up empty folders
	deleteEmptyIngressFolders(serverConfig.IngressPath)

	// Recalculate word cloud after ingestion
	db.UpdateJobProgress(jobID, 95, "Updating word cloud")
	Logger.Info("Recalculating word cloud after ingestion")
	if err := db.RecalculateAllWordFrequencies(); err != nil {
		Logger.Error("Word cloud recalculation failed after ingestion", "error", err)
	}

	// Complete the job
	result := fmt.Sprintf(`{"filesProcessed": %d, "filesTotal": %d, "errors": %d, "duplicates": %d}`, processedFiles, totalFiles, errorCount, duplicateCount)
	if err := db.CompleteJob(jobID, result); err != nil {
		Logger.Error("Failed to mark job as complete", "error", err)
	}

	Logger.Info("Ingestion job completed", "jobID", jobID, "processed", processedFiles, "total", totalFiles, "errors", errorCount, "duplicates", duplicateCount)
}

// cleanupJobFuncWithTracking performs database cleanup with job tracking
func (serverHandler *ServerHandler) cleanupJobFuncWithTracking(db database.Repository, jobID ulid.ULID) {
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered in cleanup job", "panic", r, "jobID", jobID)
			db.UpdateJobError(jobID, fmt.Sprintf("Panic: %v", r))
		}
	}()

	// Mark job as running
	db.UpdateJobStatus(jobID, database.JobStatusRunning, "Fetching documents from database")

	// Get all documents from database
	documentsPtr, err := database.FetchAllDocuments(db)
	if err != nil {
		Logger.Error("Failed to fetch documents for cleanup", "error", err)
		db.UpdateJobError(jobID, fmt.Sprintf("Failed to fetch documents: %v", err))
		return
	}

	if documentsPtr == nil {
		result := `{"scanned": 0, "deleted": 0, "moved": 0}`
		db.CompleteJob(jobID, result)
		return
	}

	documents := *documentsPtr
	totalDocs := len(documents)
	deletedCount := 0

	Logger.Info("Starting database cleanup", "total_documents", totalDocs)
	db.UpdateJobProgress(jobID, 10, fmt.Sprintf("Checking %d documents", totalDocs))

	// Step 1: Check each document's file existence and remove orphaned DB entries
	for i, doc := range documents {
		if doc.Path == "" {
			Logger.Warn("Document has empty path, skipping", "id", doc.StormID, "name", doc.Name)
			continue
		}

		// Update progress
		progress := 10 + int((float64(i)/float64(totalDocs))*50)
		db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Checking document %d/%d", i+1, totalDocs))

		// Check if file exists
		if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
			Logger.Info("File not found, removing from database", "path", doc.Path, "id", doc.StormID)

			// Delete from database
			if err := database.DeleteDocument(doc.ULID.String(), db); err != nil {
				Logger.Error("Failed to delete document from DB", "error", err, "id", doc.StormID)
				continue
			}
			deletedCount++
		}
	}

	// Step 2: Find orphaned files in document storage and move them to ingress
	db.UpdateJobProgress(jobID, 60, "Scanning for orphaned files")
	movedCount := 0
	orphanedFiles, err := serverHandler.findOrphanedDocuments(documents)
	if err != nil {
		Logger.Error("Failed to scan for orphaned documents", "error", err)
		// Continue with cleanup even if orphan scan fails
	} else {
		totalOrphans := len(orphanedFiles)
		for i, orphanPath := range orphanedFiles {
			progress := 60 + int((float64(i)/float64(totalOrphans))*20)
			db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Moving orphan %d/%d", i+1, totalOrphans))

			if err := serverHandler.moveOrphanToIngress(orphanPath); err != nil {
				Logger.Error("Failed to move orphaned document to ingress", "path", orphanPath, "error", err)
			} else {
				movedCount++
			}
		}
	}

	// Step 3: Recreate missing sidecar .txt files from database
	db.UpdateJobProgress(jobID, 70, "Checking sidecar .txt files")
	sidecarCount := 0
	if serverHandler.ServerConfig.UseSidecarTxt {
		Logger.Info("Checking for missing sidecar .txt files")
		lastSidecarUpdate := time.Now()
		for i, doc := range documents {
			// Skip if document was deleted in step 1
			if doc.Path == "" {
				continue
			}

			// Check if file still exists (it might have been deleted in step 1)
			if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
				continue
			}

			// Check if sidecar .txt file exists
			sidecarPath := getSidecarTxtPath(doc.Path)
			if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
				// Sidecar doesn't exist but document has text in database
				if doc.FullText != "" {
					Logger.Info("Recreating missing sidecar .txt file", "document", doc.Path, "sidecar", sidecarPath)
					if err := saveSidecarTxtFile(doc.Path, doc.FullText); err != nil {
						Logger.Error("Failed to recreate sidecar .txt file", "document", doc.Path, "error", err)
					} else {
						sidecarCount++
					}
				}
			}

			// Update progress every 5 seconds or on last item
			if time.Since(lastSidecarUpdate) >= 5*time.Second || i == totalDocs-1 {
				progress := 70 + int((float64(i+1)/float64(totalDocs))*5)
				db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Checking sidecar files %d/%d", i+1, totalDocs))
				lastSidecarUpdate = time.Now()
			}
		}
		Logger.Info("Sidecar .txt file check complete", "recreated", sidecarCount)
	}

	// Step 4: Check for missing thumbnails for PDF documents (only generate if missing)
	db.UpdateJobProgress(jobID, 75, "Checking document thumbnails")
	thumbnailCount := 0
	thumbnailsChecked := 0
	Logger.Info("Checking for missing thumbnails")
	lastThumbnailUpdate := time.Now()
	for i, doc := range documents {
		// Skip if document was deleted in step 1
		if doc.Path == "" {
			continue
		}

		// Only check thumbnails for PDF files
		if filepath.Ext(doc.Path) != ".pdf" {
			continue
		}

		// Check if file still exists
		if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
			continue
		}

		thumbnailsChecked++

		// Check if thumbnail exists - only generate if missing
		thumbnailPath := getThumbnailPath(doc.Path)
		if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
			// Thumbnail doesn't exist, generate it
			Logger.Info("Generating missing thumbnail", "document", doc.Path, "thumbnail", thumbnailPath)
			if err := saveThumbnailFile(doc.Path); err != nil {
				Logger.Error("Failed to generate thumbnail", "document", doc.Path, "error", err)
			} else {
				thumbnailCount++
			}
		}

		// Update progress every 5 seconds or on last item
		if time.Since(lastThumbnailUpdate) >= 5*time.Second || i == totalDocs-1 {
			progress := 75 + int((float64(i+1)/float64(totalDocs))*10)
			db.UpdateJobProgress(jobID, progress, fmt.Sprintf("Checking thumbnails %d/%d (generated %d)", i+1, totalDocs, thumbnailCount))
			lastThumbnailUpdate = time.Now()
		}
	}
	Logger.Info("Thumbnail check complete", "checked", thumbnailsChecked, "generated", thumbnailCount)

	// Step 5: Recalculate word cloud
	db.UpdateJobProgress(jobID, 85, "Recalculating word cloud")
	Logger.Info("Recalculating word cloud after database cleanup")
	if err := db.RecalculateAllWordFrequencies(); err != nil {
		Logger.Error("Word cloud recalculation failed after cleanup", "error", err)
	}

	// Complete the job
	result := fmt.Sprintf(`{"scanned": %d, "deleted": %d, "moved": %d, "sidecarRecreated": %d, "thumbnailsChecked": %d, "thumbnailsGenerated": %d}`, totalDocs, deletedCount, movedCount, sidecarCount, thumbnailsChecked, thumbnailCount)
	if err := db.CompleteJob(jobID, result); err != nil {
		Logger.Error("Failed to mark cleanup job as complete", "error", err)
	}

	Logger.Info("Database cleanup job completed", "jobID", jobID, "scanned", totalDocs, "deleted", deletedCount, "moved", movedCount)
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

	// Create output image path
	imageName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	imageName = filepath.Base(fmt.Sprint(imageName + ".png"))
	imageName = filepath.Join("temp", imageName)
	imageName, err = filepath.Abs(imageName)
	if err != nil {
		Logger.Error("Unable to edit absolute path string for temporary image for OCR", "fileName", fileName, "error", err)
		return nil, err
	}

	err = os.MkdirAll(filepath.Dir(imageName), os.ModePerm)
	if err != nil {
		Logger.Error("Unable to create absolute path for temporary image for OCR (permissions?)", "dir", filepath.Dir(imageName), "error", err)
		return nil, err
	}

	fileName = filepath.Clean(fileName)
	imageName = filepath.Clean(imageName)
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

	var fullText string
	var err error
	textFileName := filepath.Base(imageName)                                    //creating the path for the .txt that tesseract will output with the OCR results.
	textFileName = strings.TrimSuffix(textFileName, filepath.Ext(textFileName)) //just get the name, no extension
	fullpath := filepath.Join("temp", textFileName)
	fullpath, err = filepath.Abs(fullpath)
	if err != nil {
		Logger.Error("Unable to create full path for temp OCR File", "fullpath", fullpath)
	}
	textFileName = filepath.Clean(fullpath)
	/* 	tempOCRFile, err := os.Create(fmt.Sprintf("temp/%s", imageName))
	   	if err != nil {
	   		Logger.Error("Unable to create temp file", "path", fmt.Sprintf("temp/%s", imageName), "error", err)
	   		return nil, err
	   	} */
	tesseractArgs := []string{imageName, textFileName}                                       //outputting ocr to a txt file
	tesseractCMD := exec.Command(serverHandler.ServerConfig.TesseractPath, tesseractArgs...) //get the path to tesseract
	var stdBuffer bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &stdBuffer)

	tesseractCMD.Stdout = mw
	tesseractCMD.Stderr = mw

	err = tesseractCMD.Run()
	Logger.Debug("Tesseract Command Run was", "command", tesseractCMD.String())
	if err != nil {
		Logger.Warn("Tesseract encountered error when attempting to OCR image, storing document without text", "imageName", imageName, "detail", stdBuffer.String())
		emptyText := ""
		return &emptyText, nil // Return empty text instead of error - document should still be saved
	}
	fileBytes, err := os.ReadFile(textFileName + ".txt")
	if err != nil {
		Logger.Warn("Unable to read OCR output file, storing document without text", "textFile", textFileName+".txt", "error", err)
		emptyText := ""
		return &emptyText, nil
	}
	fullText = string(fileBytes)
	if fullText == "" {
		Logger.Info("OCR returned empty string - document may have no recognizable text (e.g., handwritten, blank, or image-only)", "imageName", imageName)
		// Empty text is valid - return it successfully
	}
	return &fullText, nil
}

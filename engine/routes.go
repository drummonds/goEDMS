package engine

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/blevesearch/bleve"
	"github.com/drummonds/goEDMS/config"
	"github.com/drummonds/goEDMS/database"
	"github.com/labstack/echo/v4"
)

// ServerHandler will inject the variables needed into routes
type ServerHandler struct {
	DB           database.DBInterface
	SearchDB     bleve.Index
	Echo         *echo.Echo
	ServerConfig config.ServerConfig
}

/* type Node struct {
	FullPath     string  `json:"path"`
	Name         string  `json:"name"`
	Size         int64   `json:"size"`
	DateModified string  `json:"dateModified"`
	Thumbnail    string  `json:"thumbnail"`
	IsDirectory  bool    `json:"isDirectory"`
	Children     []*Node `json:"items"`
	FileExt      string  `json:"fileExt"`
	ULID         string  `json:"ulid"`
	URL          string  `json:"documentURL"`
	Parent       *Node   `json:"-"`
} */

type fullFileSystem struct {
	FileSystem []fileTreeStruct `json:"fileSystem"`
	Error      string           `json:"error"`
}

type fileTreeStruct struct {
	ID          string   `json:"id"`
	ULIDStr     string   `json:"ulid"`
	Name        string   `json:"name"`
	Size        int64    `json:"size"`
	ModDate     string   `json:"modDate"`
	Openable    bool     `json:"openable"`
	ParentID    string   `json:"parentID"`
	IsDir       bool     `json:"isDir"`
	ChildrenIDs []string `json:"childrenIDs"`
	FullPath    string   `json:"fullPath"`
	FileURL     string   `json:"fileURL"`
}

// AddDocumentViewRoutes adds all of the current documents to an echo route
func (serverHandler *ServerHandler) AddDocumentViewRoutes() error {
	documents, err := database.FetchAllDocuments(serverHandler.DB)
	if err != nil {
		return err
	}
	for _, document := range *documents {
		documentURL := "/document/view/" + document.ULID.String()
		serverHandler.Echo.File(documentURL, document.Path)
	}
	return nil
}

// DeleteFile deletes a folder or file from the database (and all children if folder) (and on disc and from bleve search if document)
func (serverHandler *ServerHandler) DeleteFile(context echo.Context) error {
	var err error
	params := context.QueryParams()
	ulidStr := params.Get("id")
	path := params.Get("path")
	path = filepath.Join(serverHandler.ServerConfig.DocumentPath, path)
	path, err = filepath.Abs(path)
	if err != nil {
		return context.JSON(http.StatusInternalServerError, err)
	}
	fmt.Println("PATH", path)
	if path == serverHandler.ServerConfig.DocumentPath { //TODO: IMPORTANT: Make this MUCH safer so we don't literally purge everything in root lol (side note, yes I did discover that the hard way)
		return context.JSON(http.StatusInternalServerError, err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		Logger.Error("Unable to get information for file", "path", path, "error", err)
		return context.JSON(http.StatusNotFound, err)
	}
	if fileInfo.IsDir() { //If a directory, just delete it and all children
		err = DeleteFile(path)
		if err != nil {
			Logger.Error("Unable to delete folder from document filesystem", "path", path, "error", err)
			return context.JSON(http.StatusInternalServerError, err)
		}
		return context.JSON(http.StatusOK, "Folder Deleted")
	}
	document, _, err := database.FetchDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("Unable to delete folder from document filesystem", "path", path, "error", err)
		return context.JSON(http.StatusNotFound, err)
	}
	err = database.DeleteDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("Unable to delete document from database", "name", document.Name, "error", err)
		return context.JSON(http.StatusNotFound, err)
	}
	err = DeleteFile(document.Path)
	if err != nil {
		Logger.Error("Unable to delete document from file system", "path", document.Path, "error", err)
		return context.JSON(http.StatusNotFound, err)
	}
	err = database.DeleteDocumentFromSearch(document, serverHandler.SearchDB)
	if err != nil {
		Logger.Error("Unable to delete document from bleve search", "path", document.Path, "error", err)
		return context.JSON(http.StatusNotFound, err)
	}
	return context.JSON(http.StatusOK, "Document Deleted")
}

// UploadDocuments handles documents uploaded from the frontend
func (serverHandler *ServerHandler) UploadDocuments(context echo.Context) error {
	request := context.Request()
	uploadPath := request.FormValue("path")
	file, fileHeader, err := request.FormFile("file")
	if err != nil {
		fmt.Println("Problem finding file, ", err)
		return err
	}
	defer file.Close()
	//Upload it to the ingress folder so if there is an issue it will stick there and not in the documents folder which will cause issues.
	path := filepath.ToSlash(serverHandler.ServerConfig.IngressPath + "/" + uploadPath + fileHeader.Filename)
	_, err = os.Stat(filepath.Dir(path)) //since this is the ingress folder we MAY need to create the directory path.
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
			if err != nil {
				Logger.Error("Unable to create filepath for upload", "path", path, "error", err)
				return err
			}
		}
	}
	Logger.Debug("Creating path for file upload to ingress", "dir", filepath.Dir(path))
	body, err := io.ReadAll(file) //get the file, write it to the filesystem
	err = os.WriteFile(path, body, 0644)
	if err != nil {
		Logger.Error("Unable to write uploaded file", "path", path, "error", err)
		return err
	}
	serverHandler.ingressDocument(path, "upload") //ingress the document into the database
	return context.JSON(http.StatusOK, path)
}

// MoveDocuments will accept an API call from the frontend to move a document or documents
func (serverHandler *ServerHandler) MoveDocuments(context echo.Context) error {
	var docIDs url.Values
	var newFolder string
	docIDs = context.QueryParams()
	newFolder = docIDs.Get("folder")
	fmt.Println("newfolder: ", newFolder)
	fmt.Println("ID's: ", docIDs["id"])
	for _, docID := range docIDs["id"] { //fetching all the needed documents
		//document, httpStatus, err := database.FetchDocument(docID, serverHandler.DB)
		//if err != nil {
		//	Logger.Error("GetDocument API call failed (MoveDocuments)", "error", err)
		//	return context.JSON(httpStatus, err)
		//}
		//foundDocuments = append(foundDocuments, document)
		httpStatus, err := database.UpdateDocumentField(docID, "Folder", newFolder, serverHandler.DB)
		if err != nil {
			Logger.Error("GetDocument API call failed (MoveDocuments)", "error", err)
			return context.JSON(httpStatus, err)
		}
	}
	return context.JSON(http.StatusOK, "Ok")
}

// SearchDocuments will take the search terms and search all documents
func (serverHandler *ServerHandler) SearchDocuments(context echo.Context) error {
	searchParams := context.QueryParams()
	searchTerm := searchParams.Get("term")
	if searchTerm == "" {
		return context.JSON(http.StatusNotFound, "Empty search term")
	}
	var phraseSearch bool
	var searchResults *bleve.SearchResult
	var err error
	for _, char := range searchTerm { //TODO, right now both phrase and single term go to same place
		if unicode.IsSpace(char) { //if there is a space in the result, do a phrase search
			Logger.Debug("Found space in search term, converting to phrase", "searchTerm", searchTerm)
			phraseSearch = true
			searchResults, err = SearchGeneralPhrase(searchTerm, serverHandler.SearchDB)
			if err != nil {
				Logger.Error("Search failed", "error", err)
				return context.JSON(http.StatusInternalServerError, err)
			}
		}
	}
	if !phraseSearch { //if no space found in search term
		Logger.Debug("Performing Single Term Search", "searchTerm", searchTerm)
		searchResults, err = SearchGeneralPhrase(searchTerm, serverHandler.SearchDB)
		if err != nil {
			Logger.Error("Search returned an error", "error", err, "searchTerm", searchTerm)
			return context.JSON(http.StatusInternalServerError, err)
		}
	}
	if searchResults.Total == 0 {
		Logger.Info("Search returned no results", "searchTerm", searchTerm)
		return context.JSON(http.StatusNoContent, nil)
	}
	documents, err := ParseSearchResults(searchResults, serverHandler.DB)
	if err != nil {
		Logger.Error("Unable to convert results to documents", "error", err)
		return context.JSON(http.StatusInternalServerError, err)
	}
	fullResults, err := convertDocumentsToFileTree(documents)
	if err != nil {
		Logger.Error("Unable to get documents from search", "error", err)
		return context.JSON(http.StatusNotFound, err)
	}
	return context.JSON(http.StatusOK, fullResults)
}

// GetDocument will return a document by ULID
func (serverHandler *ServerHandler) GetDocument(context echo.Context) error {
	ulidStr := context.Param("id")
	document, httpStatus, err := database.FetchDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("GetDocument API call failed", "error", err)
		return context.JSON(httpStatus, err)
	}
	return context.JSON(httpStatus, document)

}

// GetDocumentFileSystem will scan the document folder and get the complete tree to send to the frontend
func (serverHandler *ServerHandler) GetDocumentFileSystem(context echo.Context) error {
	fileSystem, err := fileTree(serverHandler.ServerConfig.DocumentPath, serverHandler.DB)
	if err != nil {
		return err
	}
	//fileSystem := fileSystem{FolderTree: *folderTree, FileTree: *documents}
	return context.JSON(http.StatusOK, fileSystem)

}

func convertDocumentsToFileTree(documents []database.Document) (fullFileTree *[]fileTreeStruct, err error) {
	var fileTree []fileTreeStruct
	var currentFile fileTreeStruct
	for _, document := range documents {
		documentInfo, err := os.Stat(document.Path)
		if err != nil {
			return nil, err
		}
		currentFile.ID = document.ULID.String()
		currentFile.ULIDStr = currentFile.ID
		currentFile.Size = documentInfo.Size()
		currentFile.Name = document.Name
		currentFile.Openable = true
		currentFile.ModDate = documentInfo.ModTime().String()
		currentFile.IsDir = false
		currentFile.FullPath = document.Path
		currentFile.FileURL = document.URL
		currentFile.ParentID = "SearchResults"
		fileTree = append(fileTree, currentFile)
	}
	childrenIDs := func() []string {
		var ids []string
		for _, file := range fileTree {
			ids = append(ids, file.Name)
		}
		return ids
	}
	rootDir := fileTreeStruct{ //creating a fake root directory to display results in
		ID:          "SearchResults",
		Size:        0,
		Name:        "Search Results",
		Openable:    true,
		ModDate:     time.Now().String(),
		IsDir:       true,
		FullPath:    "null",
		ChildrenIDs: childrenIDs(),
	}
	fileTree = append([]fileTreeStruct{rootDir}, fileTree...)
	return &fileTree, nil
}

func fileTree(rootPath string, db database.DBInterface) (fileTree *fullFileSystem, err error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}
	var fullFileTree fullFileSystem
	var currentFile fileTreeStruct

	walkFunc := func(path string, info os.FileInfo, err error) error {
		newTime := time.Now()
		if err != nil {
			return err
		}
		// Reset currentFile struct for each iteration to avoid data pollution
		currentFile = fileTreeStruct{}
		currentFile.Name = info.Name()
		currentFile.FullPath = path

		for _, fileElement := range fullFileTree.FileSystem { //Find the parentID
			if fileElement.FullPath == filepath.Dir(path) {
				currentFile.ParentID = fileElement.ID
			}
		}

		if info.IsDir() {
			ULID, err := database.CalculateUUID(newTime)
			//fmt.Println("New ULID for: ", path, ULID.String())
			if err != nil {
				return err
			}
			currentFile.ID = ULID.String() + filepath.Base(path) //TODO, should I store the entire filesystem layout?  Most likely yes?
			currentFile.IsDir = true
			currentFile.Openable = true
			childIDs, err := getChildrenIDs(path)
			if err != nil {
				return err
			}
			currentFile.ChildrenIDs = *childIDs
			/* 			if path == rootPath {
				fullFileTree = append(fullFileTree, currentFile)
				return nil
			} */
		} else { //for files process size, moddate, ulid
			currentFile.Size = info.Size()
			currentFile.Openable = true
			currentFile.IsDir = false
			currentFile.ModDate = info.ModTime().String()

			document, err := database.FetchDocumentFromPath(path, db)
			if err != nil {
				fullFileTree.Error = fmt.Sprintf("Document found in directory without database entry, please investigate: %s", path)
			}
			currentFile.FileURL = document.URL
			currentFile.ID = document.ULID.String()
			currentFile.ULIDStr = document.ULID.String()
		}

		fullFileTree.FileSystem = append(fullFileTree.FileSystem, currentFile)
		return nil
	}
	err = filepath.Walk(absRoot, walkFunc)
	if err != nil {
		return nil, err
	}
	return &fullFileTree, nil
}

func getChildrenIDs(rootPath string) (*[]string, error) {
	results, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}
	var childIDs []string
	for _, result := range results {
		childIDs = append(childIDs, result.Name())
	}
	return &childIDs, nil

}

// GetLatestDocuments gets the latest documents that were ingressed
func (serverHandler *ServerHandler) GetLatestDocuments(context echo.Context) error {
	// Get page parameter (default to 1)
	page := 1
	if pageParam := context.QueryParam("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	// Fixed page size of 20
	pageSize := 20

	// Get paginated documents and total count
	documents, totalCount, err := serverHandler.DB.GetNewestDocumentsWithPagination(page, pageSize)
	if err != nil {
		Logger.Error("Can't find latest documents", "error", err)
		return context.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to fetch documents",
		})
	}

	// Calculate pagination metadata
	totalPages := (totalCount + pageSize - 1) / pageSize // Ceiling division

	return context.JSON(http.StatusOK, map[string]interface{}{
		"documents":   documents,
		"page":        page,
		"pageSize":    pageSize,
		"totalCount":  totalCount,
		"totalPages":  totalPages,
		"hasNext":     page < totalPages,
		"hasPrevious": page > 1,
	})
}

// GetFolder fetches all the documents in the folder
func (serverHandler *ServerHandler) GetFolder(context echo.Context) error {
	folderName := context.Param("folder")

	folderContents, err := database.FetchFolder(folderName, serverHandler.DB)
	if err != nil {
		Logger.Error("API GetFolder call failed", "error", err)
		return err
	}
	return context.JSON(http.StatusOK, folderContents)

}

// CreateFolder creates a folder in the document tree
func (serverHandler *ServerHandler) CreateFolder(context echo.Context) error {
	params := context.QueryParams()
	folderName := params.Get("folder")
	folderPath := params.Get("path")
	fullFolder := filepath.Join(folderPath, folderName)
	fullFolder = filepath.Join(serverHandler.ServerConfig.DocumentPath, fullFolder)
	fullFolder = filepath.Clean(fullFolder)
	fmt.Println("fullfolder: ", fullFolder, " folderName: ", folderName, "Path: ", folderPath)
	err := os.Mkdir(fullFolder, os.ModePerm)
	if err != nil {
		Logger.Error("Unable to create directory", "error", err)
		return err
	}
	serverHandler.GetDocumentFileSystem(context)
	return context.JSON(http.StatusOK, fullFolder)
}

//TODO: for a different react frontend that requires a nested JSON structure, also used for recreating dir structure in ingress
/* func folderTree(rootPath string) (folderTree *[]folderTreeStruct, err error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	var fullFolderTree []folderTreeStruct
	var currentFolder folderTreeStruct
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			currentFolder.ID = info.Name()
			currentFolder.Name = info.Name()
			currentFolder.IsDir = true
			currentFolder.Openable = true
			childIDs, err := getChildrenIDs(path)
			if err != nil {
				return err
			}
			currentFolder.ChildrenIDs = *childIDs
			if path == rootPath {
				fullFolderTree = append(fullFolderTree, currentFolder)
				return nil
			}
			getDir := filepath.Dir(path)
			currentFolder.ParentID = filepath.Base(getDir) //purging the end folder
			fullFolderTree = append(fullFolderTree, currentFolder)
		}
		return nil
	}
	err = filepath.Walk(absRoot, walkFunc)
	if err != nil {
		return nil, err
	}
	return &fullFolderTree, nil
} */

/* func documentFileTree(rootPath string, db *storm.DB) (result *Node, err error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}
	parents := make(map[string]*Node)
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		var document database.Document
		if !info.IsDir() {
			document, err = database.FetchDocumentFromPath(path, db)
			if err != nil {
				Logger.Error("Unable to fetch document", "path", path, "error", err)
			}
		}

		parents[path] = &Node{
			FullPath:     filepath.ToSlash(path),
			Name:         info.Name(),
			Size:         info.Size(),
			DateModified: info.ModTime().String(),
			Thumbnail:    "",
			FileExt:      filepath.Ext(path),
			ULID:         document.ULID.String(),
			URL:          document.URL,
			IsDirectory:  info.IsDir(),
			Children:     make([]*Node, 0),
		}
		return nil
	}
	if err = filepath.Walk(absRoot, walkFunc); err != nil {
		return
	}
	for path, node := range parents {
		parentPath := filepath.Dir(path)
		parent, exists := parents[parentPath]
		if !exists { // If a parent does not exist, this is the root.
			result = node
		} else {
			node.Parent = parent
			parent.Children = append(parent.Children, node)
		}
	}
	return
} */

// GetAboutInfo returns information about the application configuration
func (serverHandler *ServerHandler) GetAboutInfo(c echo.Context) error {
	// Get git commit hash
	gitVersion := "f9d84497" // This should be set at build time via ldflags

	// Determine OCR status
	ocrConfigured := serverHandler.ServerConfig.TesseractPath != ""

	// Get database type
	dbType := serverHandler.ServerConfig.DatabaseType
	if dbType == "" {
		dbType = "postgres" // default
	}

	// Parse database connection string to extract details
	connString := serverHandler.ServerConfig.DatabaseConnString
	dbHost := ""
	dbPort := ""
	dbName := ""
	isEphemeral := false

	// Check if it's an empty connection string (indicates ephemeral database)
	if connString == "" {
		isEphemeral = true
		dbHost = "ephemeral (temporary)"
		dbPort = "N/A"
		dbName = "temporary test database"
	} else {
		// Parse PostgreSQL connection string
		// Format: "host=localhost port=5432 user=postgres password=postgres dbname=goedms sslmode=disable"
		parts := strings.Split(connString, " ")
		for _, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				value := strings.TrimSpace(kv[1])
				switch key {
				case "host":
					dbHost = value
				case "port":
					dbPort = value
				case "dbname":
					dbName = value
				}
			}
		}

		// If we couldn't parse values, provide defaults
		if dbHost == "" {
			dbHost = "localhost"
		}
		if dbPort == "" {
			dbPort = "5432"
		}
		if dbName == "" {
			dbName = "goedms"
		}
	}

	aboutInfo := map[string]interface{}{
		"version":       gitVersion,
		"ocrConfigured": ocrConfigured,
		"ocrPath":       serverHandler.ServerConfig.TesseractPath,
		"databaseType":  dbType,
		"databaseHost":  dbHost,
		"databasePort":  dbPort,
		"databaseName":  dbName,
		"isEphemeral":   isEphemeral,
	}

	return c.JSON(http.StatusOK, aboutInfo)
}

// RunIngestNow triggers the ingestion process manually
func (serverHandler *ServerHandler) RunIngestNow(c echo.Context) error {
	Logger.Info("Manual ingestion triggered via API")

	// Run ingestion in a goroutine so we can return immediately
	go func() {
		serverHandler.ingressJobFunc(serverHandler.ServerConfig, serverHandler.DB, serverHandler.SearchDB)
		Logger.Info("Manual ingestion completed")
	}()

	return c.String(http.StatusOK, "Ingestion started")
}

// CleanDatabase checks all documents and removes entries for missing files,
// and moves orphaned files (not in database) back to ingress for reprocessing
func (serverHandler *ServerHandler) CleanDatabase(c echo.Context) error {
	Logger.Info("Database cleanup triggered via API")

	// Get all documents from database
	documentsPtr, err := database.FetchAllDocuments(serverHandler.DB)
	if err != nil {
		Logger.Error("Failed to fetch documents for cleanup", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "Failed to fetch documents",
			"message": err.Error(),
		})
	}

	if documentsPtr == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "No documents found",
			"scanned": 0,
			"deleted": 0,
			"moved":   0,
		})
	}

	documents := *documentsPtr
	scannedCount := len(documents)
	deletedCount := 0

	Logger.Info("Starting database cleanup", "total_documents", scannedCount)

	// Step 1: Check each document's file existence and remove orphaned DB entries
	for _, doc := range documents {
		if doc.Path == "" {
			Logger.Warn("Document has empty path, skipping", "id", doc.StormID, "name", doc.Name)
			continue
		}

		// Check if file exists
		if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
			Logger.Info("File not found, removing from database", "path", doc.Path, "id", doc.StormID)

			// Delete from database
			if err := database.DeleteDocument(doc.ULID.String(), serverHandler.DB); err != nil {
				Logger.Error("Failed to delete document from DB", "error", err, "id", doc.StormID)
				continue
			}

			// Delete from search index
			if err := serverHandler.SearchDB.Delete(doc.ULID.String()); err != nil {
				Logger.Error("Failed to delete document from search index", "error", err, "ulid", doc.ULID)
			}

			deletedCount++
		}
	}

	// Step 2: Find orphaned files in document storage and move them to ingress
	movedCount := 0
	orphanedFiles, err := serverHandler.findOrphanedDocuments(documents)
	if err != nil {
		Logger.Error("Failed to scan for orphaned documents", "error", err)
		// Continue with cleanup even if orphan scan fails
	} else {
		for _, orphanPath := range orphanedFiles {
			if err := serverHandler.moveOrphanToIngress(orphanPath); err != nil {
				Logger.Error("Failed to move orphaned document to ingress", "path", orphanPath, "error", err)
			} else {
				movedCount++
			}
		}
	}

	Logger.Info("Database cleanup completed", "scanned", scannedCount, "deleted", deletedCount, "moved", movedCount)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Cleanup completed successfully",
		"scanned": scannedCount,
		"deleted": deletedCount,
		"moved":   movedCount,
	})
}

// findOrphanedDocuments scans the document storage directory and finds files
// that are not present in the database
func (serverHandler *ServerHandler) findOrphanedDocuments(documents []database.Document) ([]string, error) {
	// Create a map of all paths in the database for quick lookup
	dbPaths := make(map[string]bool)
	for _, doc := range documents {
		if doc.Path != "" {
			dbPaths[doc.Path] = true
			// Also mark companion files as tracked
			yamlPath := doc.Path + ".yaml"
			txtPath := doc.Path + ".txt"
			dbPaths[yamlPath] = true
			dbPaths[txtPath] = true
		}
	}

	var orphanedFiles []string
	documentPath := serverHandler.ServerConfig.DocumentPath

	// Walk through the document directory
	err := filepath.Walk(documentPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			Logger.Warn("Error accessing path during orphan scan", "path", path, "error", err)
			return nil // Continue walking
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip companion files (.yaml and .txt) - they'll be handled with their main file
		ext := filepath.Ext(path)
		if ext == ".yaml" || ext == ".txt" {
			// Check if this is a companion file (base file + .yaml or .txt)
			basePath := path[:len(path)-len(ext)]
			if _, err := os.Stat(basePath); err == nil {
				// This is a companion file, skip it for now
				return nil
			}
		}

		// Check if this file is in the database
		if !dbPaths[path] {
			// Check if it's a document file type we care about
			if isProcessableDocument(path) {
				Logger.Info("Found orphaned document", "path", path)
				orphanedFiles = append(orphanedFiles, path)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return orphanedFiles, nil
}

// isProcessableDocument checks if a file is a document type that can be processed
func isProcessableDocument(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	processableExts := []string{".pdf", ".txt", ".rtf", ".doc", ".docx", ".odf", ".tiff", ".jpg", ".jpeg", ".png"}
	for _, validExt := range processableExts {
		if ext == validExt {
			return true
		}
	}
	return false
}

// moveOrphanToIngress moves an orphaned document (and its companion files) to the ingress folder
func (serverHandler *ServerHandler) moveOrphanToIngress(docPath string) error {
	ingressPath := serverHandler.ServerConfig.IngressPath
	documentPath := serverHandler.ServerConfig.DocumentPath

	// Calculate relative path to preserve folder structure
	relPath, err := filepath.Rel(documentPath, docPath)
	if err != nil {
		Logger.Error("Failed to calculate relative path", "docPath", docPath, "documentPath", documentPath, "error", err)
		relPath = filepath.Base(docPath) // Fall back to just the filename
	}

	// Create destination path in ingress folder
	destPath := filepath.Join(ingressPath, relPath)

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create ingress directory: %w", err)
	}

	// Move the main document file
	if err := os.Rename(docPath, destPath); err != nil {
		return fmt.Errorf("failed to move document: %w", err)
	}
	Logger.Info("Moved orphaned document to ingress", "from", docPath, "to", destPath)

	// Move companion .yaml file if it exists
	yamlPath := docPath + ".yaml"
	if _, err := os.Stat(yamlPath); err == nil {
		destYamlPath := destPath + ".yaml"
		if err := os.Rename(yamlPath, destYamlPath); err != nil {
			Logger.Warn("Failed to move companion .yaml file", "path", yamlPath, "error", err)
		} else {
			Logger.Info("Moved companion .yaml file", "from", yamlPath, "to", destYamlPath)
		}
	}

	// Move companion .txt file if it exists
	txtPath := docPath + ".txt"
	if _, err := os.Stat(txtPath); err == nil {
		destTxtPath := destPath + ".txt"
		if err := os.Rename(txtPath, destTxtPath); err != nil {
			Logger.Warn("Failed to move companion .txt file", "path", txtPath, "error", err)
		} else {
			Logger.Info("Moved companion .txt file", "from", txtPath, "to", destTxtPath)
		}
	}

	return nil
}

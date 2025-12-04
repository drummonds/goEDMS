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

	"github.com/drummonds/godocs/config"
	"github.com/drummonds/godocs/database"
	"github.com/drummonds/godocs/internal/build"
	"github.com/labstack/echo/v4"
)

// ServerHandler will inject the variables needed into routes
type ServerHandler struct {
	DB           database.Repository
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
	Warnings   []string         `json:"warnings,omitempty"`
}

type fileTreeStruct struct {
	ID           string   `json:"id"`
	ULIDStr      string   `json:"ulid"`
	Name         string   `json:"name"`
	Size         int64    `json:"size"`
	ModDate      string   `json:"modDate"`
	Openable     bool     `json:"openable"`
	ParentID     string   `json:"parentID"`
	IsDir        bool     `json:"isDir"`
	ChildrenIDs  []string `json:"childrenIDs"`
	FullPath     string   `json:"fullPath"`
	FileURL      string   `json:"fileURL"`
	ThumbnailURL string   `json:"thumbnailURL,omitempty"`
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
// @Summary Delete a file or folder
// @Description Deletes a document or folder from the system, including database entry and physical file
// @Tags Documents
// @Accept json
// @Produce json
// @Param id query string false "Document ULID"
// @Param path query string false "File path relative to document root"
// @Success 200 {string} string "Document Deleted" or "Folder Deleted"
// @Failure 404 {object} map[string]interface{} "File not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /document [delete]
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
	// PostgreSQL full-text search index is automatically updated via trigger when document is deleted
	return context.JSON(http.StatusOK, "Document Deleted")
}

// UploadDocuments handles documents uploaded from the frontend
// @Summary Upload a document
// @Description Upload a new document file to the ingress folder for processing
// @Tags Documents
// @Accept multipart/form-data
// @Produce json
// @Param path formData string false "Upload path (relative to ingress folder)"
// @Param file formData file true "Document file to upload"
// @Success 200 {string} string "Path to uploaded file"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /document/upload [post]
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
// @Summary Move documents to a new folder
// @Description Move one or more documents to a different folder in the document tree
// @Tags Documents
// @Accept json
// @Produce json
// @Param folder query string true "Target folder path"
// @Param id query []string true "Document ULID(s) to move"
// @Success 200 {string} string "Ok"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /document/move [patch]
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

// SearchDocuments will take the search terms and search all documents using PostgreSQL full-text search
// @Summary Search documents
// @Description Search all documents using PostgreSQL full-text search
// @Tags Search
// @Accept json
// @Produce json
// @Param term query string true "Search term"
// @Success 200 {object} fullFileSystem "Search results"
// @Success 204 "No results found"
// @Failure 404 {string} string "Empty search term"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /search [get]
func (serverHandler *ServerHandler) SearchDocuments(context echo.Context) error {
	searchParams := context.QueryParams()
	searchTerm := searchParams.Get("term")
	if searchTerm == "" {
		return context.JSON(http.StatusNotFound, "Empty search term")
	}

	Logger.Debug("Performing PostgreSQL full-text search", "searchTerm", searchTerm)
	documents, err := serverHandler.DB.SearchDocuments(searchTerm)
	if err != nil {
		Logger.Error("Search failed", "error", err)
		return context.JSON(http.StatusInternalServerError, err)
	}

	if len(documents) == 0 {
		Logger.Info("Search returned no results", "searchTerm", searchTerm)
		return context.JSON(http.StatusNoContent, nil)
	}

	fullResults, err := convertDocumentsToFileTree(documents)
	if err != nil {
		Logger.Error("Unable to convert search results to file tree", "error", err)
		return context.JSON(http.StatusNotFound, err)
	}

	// Wrap the results in fullFileSystem struct to match frontend expectations
	response := fullFileSystem{
		FileSystem: *fullResults,
		Error:      "",
	}
	return context.JSON(http.StatusOK, response)
}

// ReindexSearchDocuments reindexes all documents for full-text search
// @Summary Reindex search documents
// @Description Rebuild the full-text search index for all documents
// @Tags Search
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Reindex successful"
// @Failure 500 {object} map[string]interface{} "Reindex failed"
// @Router /search/reindex [post]
func (serverHandler *ServerHandler) ReindexSearchDocuments(context echo.Context) error {
	Logger.Info("Search reindex triggered via API")

	count, err := serverHandler.DB.ReindexSearchDocuments()
	if err != nil {
		Logger.Error("Reindex failed", "error", err)
		return context.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "Reindex failed",
			"message": err.Error(),
		})
	}

	Logger.Info("Search reindex completed", "documents", count)
	return context.JSON(http.StatusOK, map[string]interface{}{
		"message":             "Search reindex completed successfully",
		"documents_reindexed": count,
	})
}

// GetDocument will return a document by ULID
// @Summary Get a document by ID
// @Description Retrieve document details by ULID
// @Tags Documents
// @Accept json
// @Produce json
// @Param id path string true "Document ULID"
// @Success 200 {object} database.Document "Document details"
// @Failure 404 {object} map[string]interface{} "Document not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /document/{id} [get]
func (serverHandler *ServerHandler) GetDocument(context echo.Context) error {
	ulidStr := context.Param("id")
	document, httpStatus, err := database.FetchDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("GetDocument API call failed", "error", err)
		return context.JSON(httpStatus, err)
	}
	return context.JSON(httpStatus, document)

}

// ViewDocument serves the actual document file by ULID
// @Summary View/download a document file
// @Description Serve the actual document file for viewing or download
// @Tags Documents
// @Produce application/pdf,image/png,image/jpeg,application/octet-stream
// @Param ulid path string true "Document ULID"
// @Success 200 {file} binary "Document file"
// @Failure 404 {object} map[string]interface{} "Document not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /document/view/{ulid} [get]
func (serverHandler *ServerHandler) ViewDocument(context echo.Context) error {
	ulidStr := context.Param("ulid")
	document, httpStatus, err := database.FetchDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("ViewDocument call failed", "error", err, "ulid", ulidStr)
		return context.JSON(httpStatus, map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Serve the file from the document path
	return context.File(document.Path)
}

// GetDocumentThumbnail serves the thumbnail image for a document
// @Summary Get document thumbnail
// @Description Get a thumbnail image for a document by its ID
// @Tags Documents
// @Produce image/jpeg
// @Param id path string true "Document ULID"
// @Success 200 {file} binary "Thumbnail image"
// @Failure 404 {object} map[string]string "Thumbnail not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/document/{id}/thumbnail [get]
func (serverHandler *ServerHandler) GetDocumentThumbnail(context echo.Context) error {
	ulidStr := context.Param("id")
	document, httpStatus, err := database.FetchDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("GetDocumentThumbnail API call failed", "error", err)
		return context.JSON(httpStatus, err)
	}

	// Get thumbnail path
	thumbnailPath := getThumbnailPath(document.Path)

	// Check if thumbnail exists
	if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
		return context.JSON(http.StatusNotFound, map[string]string{
			"error": "Thumbnail not found",
		})
	}

	// Serve the thumbnail file
	return context.File(thumbnailPath)
}

// DocumentStatus represents the status information for a document
type DocumentStatus struct {
	ULID          string `json:"ulid"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	DocumentType  string `json:"documentType"`
	HasThumbnail  bool   `json:"hasThumbnail"`
	ThumbnailURL  string `json:"thumbnailURL,omitempty"`
	HasText       bool   `json:"hasText"`
	TextLength    int    `json:"textLength"`
	TextURL       string `json:"textURL,omitempty"`
	HasTags       bool   `json:"hasTags"`
	TagCount      int    `json:"tagCount"`
	ViewURL       string `json:"viewURL"`
	IngressTime   string `json:"ingressTime"`
	FileExists    bool   `json:"fileExists"`
	FileSizeBytes int64  `json:"fileSizeBytes,omitempty"`
}

// GetDocumentStatus returns status information about a document
// @Summary Get document status
// @Description Get status information including thumbnail, text, and tag availability
// @Tags Documents
// @Accept json
// @Produce json
// @Param id path string true "Document ULID"
// @Success 200 {object} DocumentStatus "Document status"
// @Failure 404 {object} map[string]string "Document not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/document/{id}/status [get]
func (serverHandler *ServerHandler) GetDocumentStatus(context echo.Context) error {
	ulidStr := context.Param("id")
	document, httpStatus, err := database.FetchDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("GetDocumentStatus API call failed", "error", err)
		return context.JSON(httpStatus, map[string]string{"error": err.Error()})
	}

	status := DocumentStatus{
		ULID:         document.ULID.String(),
		Name:         document.Name,
		Path:         document.Path,
		DocumentType: document.DocumentType,
		HasText:      len(document.FullText) > 0,
		TextLength:   len(document.FullText),
		ViewURL:      "/document/view/" + document.ULID.String(),
		IngressTime:  document.IngressTime.Format("2006-01-02 15:04:05"),
	}

	// Check if text is available
	if status.HasText {
		status.TextURL = "/api/document/" + document.ULID.String() + "/text"
	}

	// Check if thumbnail exists
	thumbnailPath := getThumbnailPath(document.Path)
	if _, err := os.Stat(thumbnailPath); err == nil {
		status.HasThumbnail = true
		status.ThumbnailURL = "/api/document/" + document.ULID.String() + "/thumbnail"
	}

	// Check if file exists and get size
	if fileInfo, err := os.Stat(document.Path); err == nil {
		status.FileExists = true
		status.FileSizeBytes = fileInfo.Size()
	}

	// Get tag count
	doc, err := serverHandler.DB.GetDocumentByULID(ulidStr)
	if err == nil && doc != nil {
		tags, err := serverHandler.DB.GetTagsForDocument(doc.StormID)
		if err == nil {
			status.TagCount = len(tags)
			status.HasTags = len(tags) > 0
		}
	}

	return context.JSON(http.StatusOK, status)
}

// GetDocumentText returns the extracted text content of a document
// @Summary Get document text
// @Description Get the extracted full text content of a document
// @Tags Documents
// @Accept json
// @Produce json
// @Param id path string true "Document ULID"
// @Success 200 {object} map[string]string "Document text"
// @Failure 404 {object} map[string]string "Document not found or no text available"
// @Router /api/document/{id}/text [get]
func (serverHandler *ServerHandler) GetDocumentText(context echo.Context) error {
	ulidStr := context.Param("id")
	document, httpStatus, err := database.FetchDocument(ulidStr, serverHandler.DB)
	if err != nil {
		Logger.Error("GetDocumentText API call failed", "error", err)
		return context.JSON(httpStatus, map[string]string{"error": err.Error()})
	}

	if len(document.FullText) == 0 {
		return context.JSON(http.StatusNotFound, map[string]string{
			"error": "No text available for this document",
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"ulid": document.ULID.String(),
		"name": document.Name,
		"text": document.FullText,
	})
}

// GetDocumentFileSystem will scan the document folder and get the complete tree to send to the frontend
// @Summary Get document filesystem tree
// @Description Retrieve the complete document folder structure as a tree
// @Tags Documents
// @Accept json
// @Produce json
// @Success 200 {object} fullFileSystem "Complete filesystem tree"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /documents/filesystem [get]
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

		// Check if thumbnail exists and add URL
		thumbnailPath := getThumbnailPath(document.Path)
		if _, err := os.Stat(thumbnailPath); err == nil {
			// Thumbnail exists, create URL for it
			currentFile.ThumbnailURL = "/api/document/" + document.ULID.String() + "/thumbnail"
		}

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

func fileTree(rootPath string, db database.Repository) (fileTree *fullFileSystem, err error) {
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
				// Skip this file but add a warning and continue processing other files
				warning := fmt.Sprintf("Document found in directory without database entry, please investigate: %s", path)
				fullFileTree.Warnings = append(fullFileTree.Warnings, warning)
				return nil // Skip this file, continue with next
			}
			currentFile.FileURL = document.URL
			currentFile.ID = document.ULID.String()
			currentFile.ULIDStr = document.ULID.String()

			// Check if thumbnail exists and add URL
			thumbnailPath := getThumbnailPath(path)
			if _, err := os.Stat(thumbnailPath); err == nil {
				// Thumbnail exists, create URL for it
				currentFile.ThumbnailURL = "/api/document/" + document.ULID.String() + "/thumbnail"
			}
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
// @Summary Get latest documents
// @Description Retrieve the most recently ingested documents with pagination
// @Tags Documents
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Success 200 {object} map[string]interface{} "Paginated documents with metadata"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /documents/latest [get]
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

	// Add thumbnail URLs to documents
	type DocumentWithThumbnail struct {
		database.Document
		ThumbnailURL string `json:"thumbnailURL,omitempty"`
	}

	documentsWithThumbnails := make([]DocumentWithThumbnail, len(documents))
	for i, doc := range documents {
		documentsWithThumbnails[i] = DocumentWithThumbnail{Document: doc}

		// Check if thumbnail exists and add URL
		thumbnailPath := getThumbnailPath(doc.Path)
		if _, err := os.Stat(thumbnailPath); err == nil {
			documentsWithThumbnails[i].ThumbnailURL = "/api/document/" + doc.ULID.String() + "/thumbnail"
		}
	}

	// Calculate pagination metadata
	totalPages := (totalCount + pageSize - 1) / pageSize // Ceiling division

	return context.JSON(http.StatusOK, map[string]interface{}{
		"documents":   documentsWithThumbnails,
		"page":        page,
		"pageSize":    pageSize,
		"totalCount":  totalCount,
		"totalPages":  totalPages,
		"hasNext":     page < totalPages,
		"hasPrevious": page > 1,
	})
}

// GetFolder fetches all the documents in the folder
// @Summary Get folder contents
// @Description Retrieve all documents in a specific folder
// @Tags Folders
// @Accept json
// @Produce json
// @Param folder path string true "Folder name"
// @Success 200 {array} database.Document "List of documents in folder"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /folder/{folder} [get]
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
// @Summary Create a new folder
// @Description Create a new folder in the document filesystem
// @Tags Folders
// @Accept json
// @Produce json
// @Param folder query string true "Folder name"
// @Param path query string true "Parent path"
// @Success 200 {string} string "Full folder path created"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /folder [post]
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
// @Summary Get application information
// @Description Retrieve information about the application configuration, version, and database
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Application information"
// @Router /about [get]
func (serverHandler *ServerHandler) GetAboutInfo(c echo.Context) error {

	// Determine OCR status
	ocrConfigured := serverHandler.ServerConfig.TesseractPath != ""

	// Get database type
	dbType := serverHandler.ServerConfig.DatabaseType
	dbHost := serverHandler.ServerConfig.DatabaseHost
	dbPort := serverHandler.ServerConfig.DatabasePort
	dbName := serverHandler.ServerConfig.DatabaseDbname

	// Get log level from environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "debug" // default
	}

	// Get schema version
	schemaVersion := "unknown"
	if version, err := serverHandler.DB.GetSchemaVersion(); err == nil {
		schemaVersion = version
	}

	aboutInfo := map[string]interface{}{
		"version":       build.GetVersion(),
		"ocrConfigured": ocrConfigured,
		"ocrPath":       serverHandler.ServerConfig.TesseractPath,
		"databaseType":  dbType,
		"databaseHost":  dbHost,
		"databasePort":  dbPort,
		"databaseName":  dbName,
		"ingressPath":   serverHandler.ServerConfig.IngressPath,
		"documentPath":  serverHandler.ServerConfig.DocumentPath,
		"logLevel":      logLevel,
		"schemaVersion": schemaVersion,
	}

	return c.JSON(http.StatusOK, aboutInfo)
}

// RunIngestNow triggers the ingestion process manually
// @Summary Trigger document ingestion
// @Description Manually trigger the document ingestion process to process files in the ingress folder
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Job created with job ID"
// @Router /ingest [post]
func (serverHandler *ServerHandler) RunIngestNow(c echo.Context) error {
	Logger.Info("Manual ingestion triggered via API")

	// Create a job to track the ingestion
	job, err := serverHandler.DB.CreateJob(database.JobTypeIngestion, "Starting document ingestion")
	if err != nil {
		Logger.Error("Failed to create ingestion job", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to create job",
		})
	}

	// Run ingestion in a goroutine so we can return immediately
	go func() {
		serverHandler.ingressJobFuncWithTracking(serverHandler.ServerConfig, serverHandler.DB, job.ID)
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Ingestion started",
		"jobId":   job.ID.String(),
	})
}

// CleanDatabase checks all documents and removes entries for missing files,
// and moves orphaned files (not in database) back to ingress for reprocessing
// @Summary Clean database
// @Description Remove database entries for missing files and move orphaned files to ingress
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Job created with jobId"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /clean [post]
func (serverHandler *ServerHandler) CleanDatabase(c echo.Context) error {
	Logger.Info("Database cleanup triggered via API")

	// Create a job to track the cleanup
	job, err := serverHandler.DB.CreateJob(database.JobTypeCleanup, "Starting database cleanup")
	if err != nil {
		Logger.Error("Failed to create cleanup job", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to create cleanup job",
		})
	}

	// Run cleanup in goroutine with job tracking
	go func() {
		serverHandler.cleanupJobFuncWithTracking(serverHandler.DB, job.ID)
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Database cleanup started",
		"jobId":   job.ID.String(),
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

// LogEntry represents a log entry from the frontend
type LogEntry struct {
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Attrs   map[string]interface{} `json:"attrs"`
}

// LogFromFrontend receives and logs messages from the frontend using slog
// @Summary Log from frontend
// @Description Accept log entries from the frontend and log them using slog with proper structure
// @Tags Admin
// @Accept json
// @Produce json
// @Param logEntry body LogEntry true "Log entry from frontend"
// @Success 200 {object} map[string]string "Log entry recorded"
// @Failure 400 {object} map[string]string "Invalid log format"
// @Router /api/log [post]
func (serverHandler *ServerHandler) LogFromFrontend(c echo.Context) error {
	var logEntry LogEntry

	if err := c.Bind(&logEntry); err != nil {
		Logger.Warn("Failed to parse frontend log entry", "error", err, "body", c.Request().Body)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid log format",
		})
	}

	// Log that we received a frontend log (for debugging)
	Logger.Debug("Received log from frontend",
		"level", logEntry.Level,
		"message", logEntry.Message,
		"attrs_count", len(logEntry.Attrs))

	// Convert map to slog.Attr slice for structured logging
	attrs := make([]any, 0, len(logEntry.Attrs)*2+2)
	attrs = append(attrs, "source", "frontend")

	for key, value := range logEntry.Attrs {
		attrs = append(attrs, key, value)
	}

	// Log with appropriate slog level with [FRONTEND] prefix
	prefixedMessage := "[FRONTEND] " + logEntry.Message

	switch logEntry.Level {
	case "error":
		Logger.Error(prefixedMessage, attrs...)
	case "warn":
		Logger.Warn(prefixedMessage, attrs...)
	case "info":
		Logger.Info(prefixedMessage, attrs...)
	case "debug":
		Logger.Debug(prefixedMessage, attrs...)
	default:
		Logger.Info(prefixedMessage, attrs...)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "logged",
	})
}

package database

import (
	"crypto/md5"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/drummonds/godocs/config"
	"github.com/oklog/ulid/v2"
)

// Document is all of the document information stored in the database
type Document struct {
	StormID      int // ID field (kept as StormID for backward compatibility)
	Name         string
	Path         string // full path to the file
	IngressTime  time.Time
	Folder       string
	Hash         string
	ULID         ulid.ULID // Have a smaller (than hash) id that can be used in URL's, hopefully speed things up
	DocumentType string    // type of document (pdf, txt, etc)
	FullText     string
	URL          string
}

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// Repository defines database operations
type Repository interface {
	Close() error
	SaveDocument(doc *Document) error
	GetDocumentByID(id int) (*Document, error)
	GetDocumentByULID(ulid string) (*Document, error)
	GetDocumentByPath(path string) (*Document, error)
	GetDocumentByHash(hash string) (*Document, error)
	GetNewestDocuments(limit int) ([]Document, error)
	GetNewestDocumentsWithPagination(page int, pageSize int) ([]Document, int, error)
	GetAllDocuments() ([]Document, error)
	GetDocumentsByFolder(folder string) ([]Document, error)
	DeleteDocument(ulid string) error
	UpdateDocumentURL(ulid string, url string) error
	UpdateDocumentFolder(ulid string, folder string) error
	SaveConfig(config *config.ServerConfig) error
	GetConfig() (*config.ServerConfig, error)
	SearchDocuments(searchTerm string) ([]Document, error)
	ReindexSearchDocuments() (int, error)
	// Word cloud methods
	GetTopWords(limit int) ([]WordFrequency, error)
	GetWordCloudMetadata() (*WordCloudMetadata, error)
	RecalculateAllWordFrequencies() error
	UpdateWordFrequencies(docID string) error
	// Job tracking methods
	CreateJob(jobType JobType, message string) (*Job, error)
	UpdateJobProgress(jobID ulid.ULID, progress int, currentStep string) error
	UpdateJobStatus(jobID ulid.ULID, status JobStatus, message string) error
	UpdateJobError(jobID ulid.ULID, errorMsg string) error
	CompleteJob(jobID ulid.ULID, result string) error
	GetJob(jobID ulid.ULID) (*Job, error)
	GetRecentJobs(limit, offset int) ([]Job, error)
	GetActiveJobs() ([]Job, error)
	DeleteOldJobs(olderThan time.Duration) (int, error)
	// Tag methods
	CreateTag(tag *Tag) error
	GetAllTags() ([]Tag, error)
	GetTagByID(id int) (*Tag, error)
	GetTagByName(name string) (*Tag, error)
	UpdateTag(tag *Tag) error
	DeleteTag(id int) error
	GetTagsForDocument(documentID int) ([]Tag, error)
	AddTagToDocument(documentID int, tagID int) error
	RemoveTagFromDocument(documentID int, tagID int) error
	GetTagUsageCount(tagID int) (int, error)
	// Dimension methods
	GetAllDimensions() ([]Dimension, error)
	GetDimensionByID(id int) (*Dimension, error)
	GetDimensionByName(name string) (*Dimension, error)
	GetDimensionValues(dimensionID int) ([]DimensionValue, error)
	GetDimensionValueByValue(dimensionID int, value string) (*DimensionValue, error)
	GetDocumentDimensions(documentID int) (map[string]DimensionValue, error)
	SetDocumentDimension(documentID int, dimensionID int, dimensionValueID int) error
	RemoveDocumentDimension(documentID int, dimensionID int) error
	// Schema version
	GetSchemaVersion() (string, error)
}

// FetchConfigFromDB pulls the server config from the database
func FetchConfigFromDB(db Repository) (config.ServerConfig, error) {
	serverConfig, err := db.GetConfig()
	if err != nil {
		Logger.Error("Unable to fetch server config from db", "error", err)
		return config.ServerConfig{}, err
	}
	return *serverConfig, nil
}

// WriteConfigToDB writes the serverconfig to the database for later retrieval
func WriteConfigToDB(serverConfig config.ServerConfig, db Repository) {
	serverConfig.StormID = 1 // config will be stored in bucket 1
	fmt.Printf("%+v\n", serverConfig)
	err := db.SaveConfig(&serverConfig)
	if err != nil {
		Logger.Error("Unable to write server config to database", "error", err)
	}
}

// AddNewDocument adds a new document to the database
func AddNewDocument(filePath string, fullText string, db Repository) (*Document, error) {
	serverConfig, err := FetchConfigFromDB(db)
	if err != nil {
		Logger.Error("Unable to fetch config to add new document", "filePath", filePath, "error", err)
	}
	var newDocument Document
	fileHash, err := calculateHash(filePath)
	if err != nil {
		return nil, err
	}
	duplicate := checkDuplicateDocument(fileHash, filePath, db)
	if duplicate {
		err = errors.New("Duplicate document found on import (Hash collision) ! " + filePath)
		Logger.Error("Duplicate document detected", "error", err)
		return nil, err // TODO return actual error
	}
	newTime := time.Now()
	newULID, err := CalculateUUID(newTime)
	if err != nil {
		Logger.Error("Cannot generate ULID", "filePath", filePath, "error", err)
	}

	newDocument.Name = filepath.Base(filePath)
	if serverConfig.IngressPreserve { //if we are preserving the entire path of the document generate the full path
		basePath := serverConfig.IngressPath
		newFileNameRoot := serverConfig.DocumentPath
		relativePath, err := filepath.Rel(basePath, filePath)
		if err != nil {
			return nil, err
		}
		newFilePath := filepath.Join(newFileNameRoot, relativePath)
		fmt.Println("NEW PATH: ", newFilePath)
		fmt.Println("New FOLDER", filepath.Dir(newFilePath))
		newDocument.Path = filepath.ToSlash(newFilePath)
		newDocument.Folder = filepath.Dir(newFilePath)
	} else {
		documentPath := filepath.ToSlash(serverConfig.DocumentPath + "/" + serverConfig.NewDocumentFolderRel + "/" + filepath.Base(filePath))
		newDocument.Path = documentPath
		documentFolder := filepath.ToSlash(serverConfig.DocumentPath + "/" + serverConfig.NewDocumentFolderRel)
		newDocument.Folder = documentFolder
	}
	newDocument.Hash = fileHash
	newDocument.IngressTime = newTime
	newDocument.ULID = newULID
	newDocument.DocumentType = filepath.Ext(filePath)
	newDocument.FullText = fullText
	Logger.Debug("Adding document to database", "fullText", newDocument.FullText)
	// PostgreSQL full-text search will be automatically indexed via trigger
	err = db.SaveDocument(&newDocument) // Writing it in document bucket
	if err != nil {
		Logger.Error("Unable to write document to bucket", "error", err)
		return nil, err
	}
	return &newDocument, nil
}

// FetchNewestDocuments fetches the documents that were added last
func FetchNewestDocuments(numberOf int, db Repository) ([]Document, error) {
	newestDocuments, err := db.GetNewestDocuments(numberOf)
	if err != nil {
		Logger.Error("Unable to find the latest documents", "error", err)
		return newestDocuments, err
	}
	return newestDocuments, nil
}

// FetchAllDocuments fetches all the documents in the database
func FetchAllDocuments(db Repository) (*[]Document, error) {
	allDocuments, err := db.GetAllDocuments()
	if err != nil {
		Logger.Error("Unable to find the latest documents", "error", err)
		return nil, err
	}
	return &allDocuments, nil
}

// FetchDocuments fetches an array of documents // TODO: Not fucking needed?
func FetchDocuments(docULIDSt []string, db Repository) ([]Document, int, error) {
	var foundDocuments []Document
	for _, ulidStr := range docULIDSt {
		tempDocument, err := db.GetDocumentByULID(ulidStr)
		if err != nil {
			Logger.Error("Unable to find the requested document", "error", err)
			return foundDocuments, http.StatusNotFound, err
		}
		foundDocuments = append(foundDocuments, *tempDocument)
	}
	return foundDocuments, http.StatusOK, nil
}

// UpdateDocumentField updates a single field in a document
func UpdateDocumentField(docULIDSt string, field string, newValue interface{}, db Repository) (int, error) {
	var err error

	// Handle specific field updates using type-safe methods
	switch field {
	case "URL":
		if url, ok := newValue.(string); ok {
			err = db.UpdateDocumentURL(docULIDSt, url)
		} else {
			return http.StatusBadRequest, errors.New("URL value must be a string")
		}
	case "Folder":
		if folder, ok := newValue.(string); ok {
			err = db.UpdateDocumentFolder(docULIDSt, folder)
		} else {
			return http.StatusBadRequest, errors.New("Folder value must be a string")
		}
	default:
		return http.StatusBadRequest, errors.New("unsupported field update: " + field)
	}

	if err != nil {
		Logger.Error("Unable to update document in db", "ulid", docULIDSt, "field", field, "error", err)
		return http.StatusNotFound, err
	}
	return http.StatusOK, nil
}

// FetchDocument fetches the requested document by ULID
func FetchDocument(docULIDSt string, db Repository) (Document, int, error) {
	foundDocument, err := db.GetDocumentByULID(docULIDSt)
	if err != nil {
		if err == sql.ErrNoRows {
			Logger.Error("Unable to find the requested document", "error", err)
			return Document{}, http.StatusNotFound, err
		}
		Logger.Error("Database error fetching document", "error", err)
		return Document{}, http.StatusInternalServerError, err
	}
	return *foundDocument, http.StatusOK, nil
}

// FetchDocumentFromPath fetches the document by document path
func FetchDocumentFromPath(path string, db Repository) (Document, error) {
	path = filepath.ToSlash(path) // converting to slash before search
	foundDocument, err := db.GetDocumentByPath(path)
	if err != nil {
		Logger.Error("Unable to find the requested document from path", "path", path, "error", err)
		return Document{}, err
	}
	return *foundDocument, nil
}

// FetchFolder grabs all of the documents contained in a folder
func FetchFolder(folderName string, db Repository) ([]Document, error) {
	folderContents, err := db.GetDocumentsByFolder(folderName) // TODO limit this?
	if err != nil {
		Logger.Error("Unable to find the requested folder", "error", err)
		return folderContents, err
	}
	return folderContents, nil
}

// DeleteDocument fetches the requested document by ULID
func DeleteDocument(docULIDSt string, db Repository) error {
	err := db.DeleteDocument(docULIDSt)
	if err != nil {
		Logger.Error("Unable to delete requested document", "error", err)
		return err
	}
	return nil
}

func checkDuplicateDocument(fileHash string, fileName string, db Repository) bool { // TODO: Check for duplicates before you do a shit ton of processing, why wasn't this obvious?
	document, err := db.GetDocumentByHash(fileHash)
	if err != nil || document == nil {
		Logger.Info("No record found, assume no duplicate hash", "error", err)
		return false
	}
	Logger.Info("Duplicate document found on import (Hash collision)", "fileName", fileName, "existingDocument", document.Name)
	return true
}

// calculate the hash of the incoming file
func calculateHash(fileName string) (string, error) {
	var fileHash string
	file, err := os.Open(fileName)
	if err != nil {
		return fileHash, err
	}
	defer file.Close()
	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return fileHash, err
	}
	fileHash = fmt.Sprintf("%x", hash.Sum(nil))
	return fileHash, nil
}

// CalculateUUID for the incoming file
func CalculateUUID(time time.Time) (ulid.ULID, error) {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.UnixNano())), 0)
	newULID, err := ulid.New(ulid.Timestamp(time), entropy)
	if err != nil {
		return newULID, err
	}
	return newULID, nil
}

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

	"github.com/blevesearch/bleve"
	"github.com/disintegration/imaging"
	"github.com/drummonds/goEDMS/config"
	"github.com/drummonds/goEDMS/database"
	"github.com/gen2brain/go-fitz"
	"github.com/ledongthuc/pdf"
)

func (serverHandler *ServerHandler) ingressJobFunc(serverConfig config.ServerConfig, db database.DBInterface, searchDB bleve.Index) {
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
		serverHandler.ingressDocument(filePath, "ingress")
	}
	deleteEmptyIngressFolders(serverHandler.ServerConfig.IngressPath) //after ingress clean empty folders
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
	document, err := database.AddNewDocument(filePath, fullText, serverHandler.DB, serverHandler.SearchDB) //Adds everything but the URL, that is added afterwards
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
func ingressCleanup(fileName string, document database.Document, serverConfig config.ServerConfig, db database.DBInterface) error {
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

	// Open PDF document using go-fitz
	doc, err := fitz.New(fileName)
	if err != nil {
		Logger.Error("Unable to open PDF document", "fileName", fileName, "error", err)
		return nil, err
	}
	defer doc.Close()

	// Get number of pages
	numPages := doc.NumPage()
	Logger.Debug("PDF has pages", "count", numPages)

	var images []image.Image

	// Convert each page to image at 150 DPI
	for pageNum := 0; pageNum < numPages; pageNum++ {
		img, err := doc.Image(pageNum)
		if err != nil {
			Logger.Error("Unable to render page", "page", pageNum, "error", err)
			continue
		}
		images = append(images, img)
	}

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
		Logger.Error("Tesseract encountered error when attempting to OCR image", "imageName", imageName, "detail", stdBuffer.String())
		return nil, err
	}
	fileBytes, err := os.ReadFile(textFileName + ".txt")
	fullText = string(fileBytes)
	if fullText == "" {
		Logger.Error("OCR Result returned empty string... OCR'ing the document failed", "imageName", imageName, "error", err)
		return nil, err
	}
	return &fullText, nil
}

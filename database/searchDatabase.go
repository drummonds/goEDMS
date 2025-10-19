package database

import (
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve"
)

// SetupSearchDB sets up new bleve or opens existing
func SetupSearchDB() (bleve.Index, error) {
	Logger.Info("Creating bleve index mapping")
	mapping := bleve.NewIndexMapping()
	var index bleve.Index
	Logger.Info("Checking if bleve index exists")
	_, err := os.Stat(filepath.Clean("databases/simpleEDMSIndex.bleve"))
	if os.IsNotExist(err) {
		Logger.Info("Creating new bleve index")
		index, err = bleve.New(filepath.Clean("databases/simpleEDMSIndex.bleve"), mapping)
		if err != nil {
			Logger.Error("Failed to create bleve index", "error", err)
			return index, err
		}
		Logger.Info("New bleve index created successfully")
	} else {
		Logger.Info("Opening existing bleve index")
		index, err = bleve.Open("databases/simpleEDMSIndex.bleve")
		if err != nil {
			Logger.Error("Failed to open bleve index", "error", err)
			return index, err
		}
		Logger.Info("Existing bleve index opened successfully")
	}
	return index, nil
}

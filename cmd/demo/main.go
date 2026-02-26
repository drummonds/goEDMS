package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/flosch/pongo2/v6"
	"github.com/labstack/echo/v4"
)

type MockTag struct {
	Name  string
	Color string
}

type MockDocument struct {
	ULID            string
	Name            string
	DocumentType    string
	HasThumbnail    bool
	Tags            []MockTag
	IsSelected      bool
	SelectToggleURL string
}

var mockDocs = []MockDocument{
	{
		ULID: "01J0000000AAAA", Name: "Invoice-2024-March.pdf", DocumentType: "pdf",
		HasThumbnail: true,
		Tags: []MockTag{
			{Name: "Finance", Color: "#3273dc"},
			{Name: "Tax", Color: "#ff3860"},
			{Name: "2024", Color: "#23d160"},
		},
		IsSelected:      true,
		SelectToggleURL: "/?view=flat",
	},
	{
		ULID: "01J0000000BBBB", Name: "Meeting-Notes.docx", DocumentType: "docx",
		HasThumbnail: false,
		Tags: []MockTag{
			{Name: "Work", Color: "#ffdd57"},
		},
		SelectToggleURL: "/?view=flat&sel=01J0000000AAAA,01J0000000BBBB",
	},
	{
		ULID: "01J0000000CCCC", Name: "Photo-Holiday.jpg", DocumentType: "jpg",
		HasThumbnail:    true,
		Tags:            nil,
		SelectToggleURL: "/?view=flat&sel=01J0000000AAAA,01J0000000CCCC",
	},
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func main() {
	port := flag.Int("port", 8080, "server port")
	flag.Parse()

	root, err := findProjectRoot()
	if err != nil {
		log.Fatal(err)
	}

	templateDir := filepath.Join(root, "templates")
	loader := pongo2.MustNewLocalFileSystemLoader(templateDir)
	tplSet := pongo2.NewSet("demo", loader)

	e := echo.New()
	e.HideBanner = true

	e.GET("/", func(c echo.Context) error {
		tpl, err := tplSet.FromFile("demo_card.html")
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		out, err := tpl.Execute(pongo2.Context{
			"documents":      mockDocs,
			"selected_ulids": "01J0000000AAAA",
			"app_version":    "demo",
			"active_page":    "",
		})
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.HTML(http.StatusOK, out)
	})

	e.GET("/api/document/:ulid/thumbnail", func(c echo.Context) error {
		img := image.NewRGBA(image.Rect(0, 0, 256, 256))
		col := color.RGBA{R: 180, G: 200, B: 220, A: 255}
		for y := range 256 {
			for x := range 256 {
				img.Set(x, y, col)
			}
		}
		c.Response().Header().Set("Content-Type", "image/png")
		return png.Encode(c.Response().Writer, img)
	})

	e.GET("/favicon.ico", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Demo server at http://localhost%s", addr)
	log.Fatal(e.Start(addr))
}

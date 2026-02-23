// cmd/md2html converts markdown files to Bulma-styled HTML pages.
//
// Usage:
//
//	go run cmd/md2html/main.go -src docs/internal -dst gh-pages/internal
package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed template.html
var templateFS embed.FS

type Breadcrumb struct {
	Label string
	URL   string
}

type PageData struct {
	Title       string
	Content     template.HTML
	Breadcrumbs []Breadcrumb
}

func main() {
	src := flag.String("src", "docs/internal", "source markdown directory")
	dst := flag.String("dst", "docs/internal", "destination HTML directory")
	flag.Parse()

	tmplBytes, err := templateFS.ReadFile("template.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read template: %v\n", err)
		os.Exit(1)
	}
	tmpl, err := template.New("page").Parse(string(tmplBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse template: %v\n", err)
		os.Exit(1)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	if err := os.MkdirAll(*dst, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", *dst, err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(*src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *src, err)
		os.Exit(1)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if err := convertFile(md, tmpl, *src, *dst, entry.Name()); err != nil {
			fmt.Fprintf(os.Stderr, "convert %s: %v\n", entry.Name(), err)
			os.Exit(1)
		}
	}
}

func convertFile(md goldmark.Markdown, tmpl *template.Template, src, dst, name string) error {
	content, err := os.ReadFile(filepath.Join(src, name))
	if err != nil {
		return err
	}

	title := extractTitle(content, name)

	var buf bytes.Buffer
	if err := md.Convert(content, &buf); err != nil {
		return fmt.Errorf("goldmark: %w", err)
	}

	outName := strings.TrimSuffix(name, ".md") + ".html"
	data := PageData{
		Title:   title,
		Content: template.HTML(buf.String()),
		Breadcrumbs: []Breadcrumb{
			{Label: "godocs", URL: "../index.html"},
			{Label: "Internal Docs", URL: ""},
			{Label: title, URL: ""},
		},
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return fmt.Errorf("template: %w", err)
	}

	outPath := filepath.Join(dst, outName)
	if err := os.WriteFile(outPath, out.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s -> %s\n", filepath.Join(src, name), outPath)
	return nil
}

func extractTitle(content []byte, fallback string) string {
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return strings.TrimSuffix(fallback, ".md")
}

package webapp

import (
	"encoding/json"
	"fmt"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// FileTreeNode represents a node in the file tree
type FileTreeNode struct {
	ID          string   `json:"id"`
	ULID        string   `json:"ulid"`
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

// FileSystem represents the API response
type FileSystem struct {
	FileSystem []FileTreeNode `json:"fileSystem"`
	Error      string         `json:"error"`
}

// BrowsePage displays the document file tree
type BrowsePage struct {
	app.Compo
	fileSystem   FileSystem
	currentPath  []string
	loading      bool
	error        string
	expandedDirs map[string]bool
}

// OnMount is called when the component is mounted
func (b *BrowsePage) OnMount(ctx app.Context) {
	b.loading = true
	b.expandedDirs = make(map[string]bool)
	b.fetchFileSystem(ctx)
}

// fetchFileSystem fetches the file tree from the API
func (b *BrowsePage) fetchFileSystem(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", "/documents/filesystem")

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}

				jsonData := args[0]
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				var fs FileSystem
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &fs); err != nil {
						b.error = fmt.Sprintf("Failed to parse response: %v", err)
					} else {
						b.fileSystem = fs
						// Expand root directory by default
						if len(fs.FileSystem) > 0 {
							b.expandedDirs[fs.FileSystem[0].ID] = true
						}
					}
					b.loading = false
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				b.error = "Network error"
				b.loading = false
			})
			return nil
		}))
	})
}

// toggleDir toggles a directory's expanded state
func (b *BrowsePage) toggleDir(ctx app.Context, id string) {
	b.expandedDirs[id] = !b.expandedDirs[id]
}

// getChildren returns the children of a node
func (b *BrowsePage) getChildren(parentID string) []FileTreeNode {
	var children []FileTreeNode
	for _, node := range b.fileSystem.FileSystem {
		if node.ParentID == parentID {
			children = append(children, node)
		}
	}
	return children
}

// renderNode renders a single file tree node
func (b *BrowsePage) renderNode(node FileTreeNode, depth int) app.UI {
	isExpanded := b.expandedDirs[node.ID]
	children := b.getChildren(node.ID)

	iconText := "📄"
	if node.IsDir {
		if isExpanded {
			iconText = "📂"
		} else {
			iconText = "📁"
		}
	}

	var nameUI app.UI
	if !node.IsDir && node.FileURL != "" {
		nameUI = app.A().Href(node.FileURL).Target("_blank").Text(node.Name)
	} else {
		nameUI = app.Text(node.Name)
	}

	var sizeUI app.UI
	if !node.IsDir && node.Size > 0 {
		sizeUI = app.Span().Class("tree-node-size").Text(fmt.Sprintf(" (%s)", formatBytes(node.Size)))
	}

	var childrenUI app.UI
	if node.IsDir && isExpanded && len(children) > 0 {
		childrenUI = app.Div().Class("tree-node-children").Body(
			app.Range(children).Slice(func(i int) app.UI {
				return b.renderNode(children[i], depth+1)
			}),
		)
	}

	return app.Div().
		Class("tree-node").
		Style("padding-left", fmt.Sprintf("%dpx", depth*20)).
		Body(
			app.Div().Class("tree-node-content").Body(
				app.Span().
					Class("tree-node-icon").
					Text(iconText).
					OnClick(func(ctx app.Context, e app.Event) {
						if node.IsDir {
							b.toggleDir(ctx, node.ID)
						}
					}),
				app.Span().Class("tree-node-name").Body(nameUI),
				sizeUI,
			),
			childrenUI,
		)
}

// formatBytes formats bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Render renders the browse page
func (b *BrowsePage) Render() app.UI {
	var content app.UI

	if b.loading {
		content = app.Div().Class("loading").Body(app.Text("Loading..."))
	} else if b.error != "" {
		content = app.Div().Class("error").Body(app.Text("Error: " + b.error))
	} else if b.fileSystem.Error != "" {
		content = app.Div().Class("warning").Body(app.Text("Warning: " + b.fileSystem.Error))
	} else if len(b.fileSystem.FileSystem) > 0 {
		content = app.Div().Class("file-tree").Body(b.renderNode(b.fileSystem.FileSystem[0], 0))
	} else {
		content = app.Text("No documents found")
	}

	return app.Div().
		Class("browse-page").
		Body(
			app.H2().Text("Browse Documents"),
			content,
		)
}

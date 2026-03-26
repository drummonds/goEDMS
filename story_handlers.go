package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/hum3/godocs/database"
	"github.com/flosch/pongo2/v6"
	"github.com/labstack/echo/v4"
)

// HandleStoriesPage renders the stories list page with a create form.
func HandleStoriesPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		stories, err := tr.db.GetStoriesWithMeta()
		if err != nil {
			Logger.Error("Failed to fetch stories", "error", err)
			stories = []database.StoryWithMeta{}
		}

		return tr.Render(c, "stories.html", pongo2.Context{
			"stories": stories,
		})
	}
}

// HandleNewStoryPage renders the new story form.
func HandleNewStoryPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		return tr.Render(c, "story_new.html", nil)
	}
}

// HandleCreateStory creates a new story with its tag.
func HandleCreateStory(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		title := c.FormValue("title")
		if title == "" {
			return c.Redirect(http.StatusSeeOther, "/stories/new")
		}

		story := &database.Story{
			Title:       title,
			Description: c.FormValue("description"),
		}

		if d := c.FormValue("start_date"); d != "" {
			if t, err := time.Parse("2006-01-02", d); err == nil {
				story.StartDate = &t
			}
		}
		if d := c.FormValue("end_date"); d != "" {
			if t, err := time.Parse("2006-01-02", d); err == nil {
				story.EndDate = &t
			}
		}

		if err := tr.db.CreateStory(story); err != nil {
			Logger.Error("Failed to create story", "error", err)
			return c.Redirect(http.StatusSeeOther, "/stories/new")
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", story.ID))
	}
}

// HandleEditStoryPage renders the story edit page.
func HandleEditStoryPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		story, err := tr.db.GetStoryByID(id)
		if err != nil || story == nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		tag, err := tr.db.GetTagByID(story.TagID)
		if err != nil {
			Logger.Error("Failed to fetch tag for story", "tagID", story.TagID, "error", err)
		}
		assocTags, err := tr.db.GetStoryTags(id)
		if err != nil {
			assocTags = []database.Tag{}
		}

		// Get all tags for the "add associated tag" dropdown, excluding the story's own tag and already-associated tags
		allTags, err := tr.db.GetAllTags()
		if err != nil {
			allTags = []database.Tag{}
		}
		excludeSet := map[int]bool{story.TagID: true}
		for _, t := range assocTags {
			excludeSet[t.ID] = true
		}
		availableTags := make([]database.Tag, 0)
		for _, t := range allTags {
			if !excludeSet[t.ID] {
				availableTags = append(availableTags, t)
			}
		}

		// Get documents in this story (those with the story tag)
		storyDocs, docCount, err := tr.db.GetDocumentsByTag(story.TagID, 1, 100)
		if err != nil {
			storyDocs = []database.Document{}
			docCount = 0
		}
		docsWithMeta := tr.enrichDocuments(storyDocs)

		// Format dates for HTML date inputs
		startDate := ""
		if story.StartDate != nil {
			startDate = story.StartDate.Format("2006-01-02")
		}
		endDate := ""
		if story.EndDate != nil {
			endDate = story.EndDate.Format("2006-01-02")
		}

		tagColor := "#8e44ad"
		if tag != nil {
			tagColor = tag.Color
		}

		return tr.Render(c, "story_edit.html", pongo2.Context{
			"story":          story,
			"tag":            tag,
			"tag_color":      tagColor,
			"assoc_tags":     assocTags,
			"available_tags": availableTags,
			"documents":      docsWithMeta,
			"document_count": docCount,
			"start_date":     startDate,
			"end_date":       endDate,
		})
	}
}

// HandleUpdateStory updates a story's fields.
func HandleUpdateStory(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		story, err := tr.db.GetStoryByID(id)
		if err != nil || story == nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		story.Title = c.FormValue("title")
		story.Description = c.FormValue("description")

		story.StartDate = nil
		if d := c.FormValue("start_date"); d != "" {
			if t, err := time.Parse("2006-01-02", d); err == nil {
				story.StartDate = &t
			}
		}
		story.EndDate = nil
		if d := c.FormValue("end_date"); d != "" {
			if t, err := time.Parse("2006-01-02", d); err == nil {
				story.EndDate = &t
			}
		}

		// Update tag color if provided
		if color := c.FormValue("color"); color != "" {
			tag, err := tr.db.GetTagByID(story.TagID)
			if err != nil {
				Logger.Error("Failed to fetch tag for story color update", "tagID", story.TagID, "error", err)
			} else if tag != nil {
				tag.Color = color
				if err := tr.db.UpdateTag(tag); err != nil {
					Logger.Error("Failed to update story tag color", "tagID", tag.ID, "error", err)
				}
			}
		}

		if err := tr.db.UpdateStory(story); err != nil {
			Logger.Error("Failed to update story", "id", id, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
	}
}

// HandleDeleteStory deletes a story and its tag.
func HandleDeleteStory(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		if err := tr.db.DeleteStory(id); err != nil {
			Logger.Error("Failed to delete story", "id", id, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, "/stories")
	}
}

// HandleAddStoryTag adds an associated tag to a story.
func HandleAddStoryTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		tagID, err := strconv.Atoi(c.FormValue("tag_id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
		}

		if err := tr.db.AddStoryTag(id, tagID); err != nil {
			Logger.Error("Failed to add story tag", "storyID", id, "tagID", tagID, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
	}
}

// HandleRemoveStoryTag removes an associated tag from a story.
func HandleRemoveStoryTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		tagID, err := strconv.Atoi(c.Param("tagId"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
		}

		if err := tr.db.RemoveStoryTag(id, tagID); err != nil {
			Logger.Error("Failed to remove story tag", "storyID", id, "tagID", tagID, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
	}
}

// HandleAddDocumentToStory adds a document to a story.
func HandleAddDocumentToStory(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		ulidStr := c.FormValue("ulid")
		doc, _, err := database.FetchDocument(ulidStr, tr.db)
		if err != nil {
			Logger.Error("Failed to fetch document for story", "ulid", ulidStr, "error", err)
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
		}

		if err := tr.db.AddDocumentToStory(doc.ID, id); err != nil {
			Logger.Error("Failed to add document to story", "docID", doc.ID, "storyID", id, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
	}
}

// HandleConvertTagToStory converts an existing tag into a story.
func HandleConvertTagToStory(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		tagID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/tags")
		}

		// Check that tag isn't already a story
		existingStory, _ := tr.db.GetStoryByTagID(tagID)
		if existingStory != nil {
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", existingStory.ID))
		}

		story, err := tr.db.ConvertTagToStory(tagID)
		if err != nil {
			Logger.Error("Failed to convert tag to story", "tagID", tagID, "error", err)
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/tags/%d/edit", tagID))
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", story.ID))
	}
}

// HandleConvertStoryToTag converts a story back to a regular tag.
func HandleConvertStoryToTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		story, err := tr.db.GetStoryByID(id)
		if err != nil || story == nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		tagID := story.TagID
		if err := tr.db.ConvertStoryToTag(id); err != nil {
			Logger.Error("Failed to convert story to tag", "storyID", id, "error", err)
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/tags/%d/edit", tagID))
	}
}

// HandleRemoveDocumentFromStory removes a document from a story.
func HandleRemoveDocumentFromStory(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/stories")
		}

		ulidStr := c.Param("ulid")
		doc, _, err := database.FetchDocument(ulidStr, tr.db)
		if err != nil {
			Logger.Error("Failed to fetch document for story removal", "ulid", ulidStr, "error", err)
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
		}

		if err := tr.db.RemoveDocumentFromStory(doc.ID, id); err != nil {
			Logger.Error("Failed to remove document from story", "docID", doc.ID, "storyID", id, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/stories/%d/edit", id))
	}
}

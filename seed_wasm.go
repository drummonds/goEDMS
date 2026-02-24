//go:build js && wasm

package main

import (
	"time"

	godocshash "github.com/drummonds/godocs-hash"
	"github.com/drummonds/godocs/database"
	"github.com/oklog/ulid/v2"
)

// seedDemoData populates the in-memory database with demo documents and tags
func seedDemoData(db *database.MemDB) {
	now := time.Now()

	// Create demo tags
	tags := []database.Tag{
		{Name: "Invoice", Color: "#e74c3c", Description: "Financial invoices"},
		{Name: "Personal", Color: "#3498db", Description: "Personal documents"},
		{Name: "Report", Color: "#2ecc71", Description: "Business reports"},
		{Name: "Technical", Color: "#9b59b6", Description: "Technical documentation"},
		{Name: "Archive", Color: "#95a5a6", Description: "Archived items"},
	}
	for i := range tags {
		tags[i].CreatedAt = now
		tags[i].UpdatedAt = now
		if err := db.CreateTag(&tags[i]); err != nil {
			Logger.Error("Failed to create demo tag", "name", tags[i].Name, "error", err)
		}
	}

	// Create demo documents
	demoDocuments := []struct {
		name     string
		folder   string
		docType  string
		text     string
		tagNames []string
	}{
		{
			name:     "2024-Q4-Financial-Report.pdf",
			folder:   "/documents/finance",
			docType:  ".pdf",
			text:     "Quarterly financial report for Q4 2024. Revenue increased by 15% compared to the previous quarter. Operating expenses remained stable. Net profit margin improved to 12.3%.",
			tagNames: []string{"Report", "Invoice"},
		},
		{
			name:     "Server-Architecture-Overview.pdf",
			folder:   "/documents/technical",
			docType:  ".pdf",
			text:     "System architecture diagram showing the three-tier architecture: client browser, web server (Go + Echo), and PostgreSQL database. Includes load balancer configuration and deployment topology.",
			tagNames: []string{"Technical", "Report"},
		},
		{
			name:     "Invoice-2024-001.pdf",
			folder:   "/documents/finance",
			docType:  ".pdf",
			text:     "Invoice #2024-001 from Acme Corp for consulting services. Amount: $5,000.00. Payment terms: Net 30. Due date: January 15, 2025.",
			tagNames: []string{"Invoice"},
		},
		{
			name:     "Meeting-Notes-Jan-2025.pdf",
			folder:   "/documents/notes",
			docType:  ".pdf",
			text:     "Meeting notes from January 2025 team standup. Topics discussed: project timeline, resource allocation, and upcoming milestones. Action items assigned to team members.",
			tagNames: []string{"Personal"},
		},
		{
			name:     "Go-Best-Practices-Guide.pdf",
			folder:   "/documents/technical",
			docType:  ".pdf",
			text:     "Comprehensive guide to Go programming best practices. Covers error handling, concurrency patterns, testing strategies, and project structure. Includes examples of idiomatic Go code.",
			tagNames: []string{"Technical"},
		},
		{
			name:     "Holiday-Photos-2024.jpg",
			folder:   "/documents/personal",
			docType:  ".jpg",
			text:     "Collection of holiday photographs from summer 2024 vacation.",
			tagNames: []string{"Personal", "Archive"},
		},
		{
			name:     "Database-Migration-Plan.pdf",
			folder:   "/documents/technical",
			docType:  ".pdf",
			text:     "Plan for migrating from Bun ORM to raw SQL with database/sql. Steps include: schema extraction, query rewriting, migration script creation, and testing strategy. Target: PostgreSQL and pglike compatibility.",
			tagNames: []string{"Technical"},
		},
		{
			name:     "Invoice-2024-002.pdf",
			folder:   "/documents/finance",
			docType:  ".pdf",
			text:     "Invoice #2024-002 from Widget Co for hardware supplies. Amount: $2,350.00. Payment terms: Net 15. Includes shipping and handling.",
			tagNames: []string{"Invoice"},
		},
	}

	// Build tag name → ID lookup
	tagMap := make(map[string]int)
	allTags, _ := db.GetAllTags()
	for _, t := range allTags {
		tagMap[t.Name] = t.ID
	}

	for i, dd := range demoDocuments {
		docTime := now.Add(-time.Duration(len(demoDocuments)-i) * 24 * time.Hour)
		id, _ := ulid.New(ulid.Timestamp(docTime), ulid.DefaultEntropy())
		hash := godocshash.HashBytes([]byte(dd.name + dd.text))

		docDate := docTime
		doc := &database.Document{
			Name:         dd.name,
			Path:         dd.folder + "/" + dd.name,
			IngressTime:  docTime,
			Folder:       dd.folder,
			Hash:         hash,
			ULID:         id,
			DocumentType: dd.docType,
			FullText:     dd.text,
			DocumentDate: &docDate,
		}

		if err := db.SaveDocument(doc); err != nil {
			Logger.Error("Failed to seed document", "name", dd.name, "error", err)
			continue
		}

		// Assign tags
		for _, tagName := range dd.tagNames {
			if tagID, ok := tagMap[tagName]; ok {
				db.AddTagToDocument(doc.ID, tagID)
			}
		}
	}

	// Create a demo story
	story := &database.Story{
		Title:       "Q4 2024 Finance Review",
		Description: "All documents related to Q4 2024 financial review and audit preparation.",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.CreateStory(story); err != nil {
		Logger.Error("Failed to create demo story", "error", err)
	} else {
		// Add finance-tagged docs to the story
		allDocs, _ := db.GetAllDocuments()
		for _, doc := range allDocs {
			docTags, _ := db.GetTagsForDocument(doc.ID)
			for _, t := range docTags {
				if t.Name == "Invoice" || t.Name == "Report" {
					db.AddDocumentToStory(doc.ID, story.ID)
					break
				}
			}
		}
	}

	Logger.Info("Demo data seeded", "documents", len(demoDocuments), "tags", len(tags))
}

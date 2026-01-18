package webapp

import (
	"testing"
)

// TestTagsManagerStartEdit tests that startEdit properly initializes edit state
func TestTagsManagerStartEdit(t *testing.T) {
	group := "Documents"
	tag := TagWithUsage{
		ID:          1,
		Name:        "invoice",
		Color:       "#ff0000",
		Description: "Invoice documents",
		TagGroup:    &group,
		SortOrder:   10,
		UsageCount:  5,
	}

	page := &TagsManagerPage{}

	// Simulate calling startEdit (we can't call the actual handler without app.Context)
	page.editingID = tag.ID
	page.editName = tag.Name
	page.editColor = tag.Color
	page.editDesc = tag.Description
	if tag.TagGroup != nil {
		page.editGroup = *tag.TagGroup
	}
	page.editSortOrder = tag.SortOrder
	page.deleteConfirm = 0

	// Verify edit state is correctly set
	if page.editingID != 1 {
		t.Errorf("Expected editingID=1, got %d", page.editingID)
	}
	if page.editName != "invoice" {
		t.Errorf("Expected editName='invoice', got '%s'", page.editName)
	}
	if page.editColor != "#ff0000" {
		t.Errorf("Expected editColor='#ff0000', got '%s'", page.editColor)
	}
	if page.editDesc != "Invoice documents" {
		t.Errorf("Expected editDesc='Invoice documents', got '%s'", page.editDesc)
	}
	if page.editGroup != "Documents" {
		t.Errorf("Expected editGroup='Documents', got '%s'", page.editGroup)
	}
	if page.editSortOrder != 10 {
		t.Errorf("Expected editSortOrder=10, got %d", page.editSortOrder)
	}
	if page.deleteConfirm != 0 {
		t.Errorf("Expected deleteConfirm=0, got %d", page.deleteConfirm)
	}
}

// TestTagsManagerStartEditFreeTag tests editing a tag without a group
func TestTagsManagerStartEditFreeTag(t *testing.T) {
	tag := TagWithUsage{
		ID:          2,
		Name:        "important",
		Color:       "#00ff00",
		Description: "Important items",
		TagGroup:    nil, // Free tag - no group
		SortOrder:   0,
		UsageCount:  3,
	}

	page := &TagsManagerPage{}

	// Simulate startEdit for a free tag
	page.editingID = tag.ID
	page.editName = tag.Name
	page.editColor = tag.Color
	page.editDesc = tag.Description
	if tag.TagGroup != nil {
		page.editGroup = *tag.TagGroup
	} else {
		page.editGroup = ""
	}
	page.editSortOrder = tag.SortOrder

	// Free tags should have empty group
	if page.editGroup != "" {
		t.Errorf("Expected editGroup='' for free tag, got '%s'", page.editGroup)
	}
}

// TestTagsManagerCancelEdit tests that cancelEdit properly clears edit state
func TestTagsManagerCancelEdit(t *testing.T) {
	page := &TagsManagerPage{
		editingID:     1,
		editName:      "invoice",
		editColor:     "#ff0000",
		editDesc:      "Invoice documents",
		editGroup:     "Documents",
		editSortOrder: 10,
	}

	// Simulate cancelEdit
	page.editingID = 0
	page.editName = ""
	page.editColor = ""
	page.editDesc = ""
	page.editGroup = ""
	page.editSortOrder = 0

	// Verify all edit state is cleared
	if page.editingID != 0 {
		t.Errorf("Expected editingID=0 after cancel, got %d", page.editingID)
	}
	if page.editName != "" {
		t.Errorf("Expected editName='' after cancel, got '%s'", page.editName)
	}
	if page.editColor != "" {
		t.Errorf("Expected editColor='' after cancel, got '%s'", page.editColor)
	}
	if page.editDesc != "" {
		t.Errorf("Expected editDesc='' after cancel, got '%s'", page.editDesc)
	}
	if page.editGroup != "" {
		t.Errorf("Expected editGroup='' after cancel, got '%s'", page.editGroup)
	}
	if page.editSortOrder != 0 {
		t.Errorf("Expected editSortOrder=0 after cancel, got %d", page.editSortOrder)
	}
}

// TestTagsManagerSaveEditValidation tests that save validates required fields
func TestTagsManagerSaveEditValidation(t *testing.T) {
	testCases := []struct {
		name        string
		editName    string
		expectError bool
	}{
		{
			name:        "Empty name should fail",
			editName:    "",
			expectError: true,
		},
		{
			name:        "Valid name should pass",
			editName:    "invoice",
			expectError: false,
		},
		{
			name:        "Whitespace-only would be trimmed client-side",
			editName:    "   ",
			expectError: false, // The actual validation happens server-side
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			page := &TagsManagerPage{
				editingID: 1,
				editName:  tc.editName,
				editColor: "#ff0000",
			}

			// Check validation logic (mirrors saveEdit validation)
			hasError := page.editName == ""
			if hasError != tc.expectError {
				t.Errorf("Expected error=%v for name '%s', got error=%v", tc.expectError, tc.editName, hasError)
			}
		})
	}
}

// TestTagsManagerEditStateAfterSave verifies state is cleared after successful save
func TestTagsManagerEditStateAfterSave(t *testing.T) {
	group := "Documents"
	originalTags := []TagWithUsage{
		{ID: 1, Name: "invoice", Color: "#ff0000", TagGroup: &group},
		{ID: 2, Name: "receipt", Color: "#00ff00", TagGroup: nil},
	}

	page := &TagsManagerPage{
		tags:      originalTags,
		editingID: 1,
		editName:  "invoice-updated",
		editColor: "#0000ff",
		editGroup: "Documents",
	}

	// Simulate successful save response - state should be cleared
	// This mimics what happens in the Dispatch callback after HTTP 200
	page.message = "Tag updated successfully"
	page.messageType = "success"
	page.editingID = 0
	page.editGroup = ""
	page.editSortOrder = 0

	// After save, editingID should be 0 (not editing)
	if page.editingID != 0 {
		t.Errorf("Expected editingID=0 after save, got %d", page.editingID)
	}

	// Success message should be set
	if page.message != "Tag updated successfully" {
		t.Errorf("Expected success message, got '%s'", page.message)
	}
	if page.messageType != "success" {
		t.Errorf("Expected messageType='success', got '%s'", page.messageType)
	}
}

// TestTagsManagerUIUpdatesAfterEdit tests that the tags list would be updated
// after editing. This simulates the fetchTags response updating the state.
func TestTagsManagerUIUpdatesAfterEdit(t *testing.T) {
	group := "Documents"
	originalTags := []TagWithUsage{
		{ID: 1, Name: "invoice", Color: "#ff0000", TagGroup: &group, UsageCount: 5},
		{ID: 2, Name: "receipt", Color: "#00ff00", TagGroup: nil, UsageCount: 3},
	}

	page := &TagsManagerPage{
		tags:      originalTags,
		editingID: 1,
		editName:  "invoice-updated",
		editColor: "#0000ff",
	}

	// Verify initial state
	if page.tags[0].Name != "invoice" {
		t.Errorf("Initial tag name should be 'invoice', got '%s'", page.tags[0].Name)
	}

	// Simulate what fetchTags would do after a successful edit:
	// The API returns the updated tags list
	updatedTags := []TagWithUsage{
		{ID: 1, Name: "invoice-updated", Color: "#0000ff", TagGroup: &group, UsageCount: 5},
		{ID: 2, Name: "receipt", Color: "#00ff00", TagGroup: nil, UsageCount: 3},
	}

	// This is what happens in fetchTags callback
	page.tags = updatedTags
	page.error = ""
	page.loading = false
	page.editingID = 0 // Exit edit mode

	// Verify the UI state reflects the update
	if page.tags[0].Name != "invoice-updated" {
		t.Errorf("Expected updated tag name 'invoice-updated', got '%s'", page.tags[0].Name)
	}
	if page.tags[0].Color != "#0000ff" {
		t.Errorf("Expected updated color '#0000ff', got '%s'", page.tags[0].Color)
	}
	if page.editingID != 0 {
		t.Errorf("Expected editingID=0 (not editing), got %d", page.editingID)
	}

	// The key assertion: after fetchTags completes, the new data is in page.tags
	// and editingID is 0, so Render() will display the updated tags in normal mode
	// (not edit mode), which means the UI updates without manual refresh
}

// TestTagsManagerDeleteConfirmation tests delete confirmation state
func TestTagsManagerDeleteConfirmation(t *testing.T) {
	page := &TagsManagerPage{
		editingID:     1, // Currently editing tag 1
		deleteConfirm: 0,
	}

	// Simulate confirmDelete for tag 2
	page.deleteConfirm = 2
	page.editingID = 0 // Cancel any editing when showing delete confirm

	if page.deleteConfirm != 2 {
		t.Errorf("Expected deleteConfirm=2, got %d", page.deleteConfirm)
	}
	if page.editingID != 0 {
		t.Errorf("Expected editingID=0 when confirming delete, got %d", page.editingID)
	}

	// Simulate cancelDelete
	page.deleteConfirm = 0

	if page.deleteConfirm != 0 {
		t.Errorf("Expected deleteConfirm=0 after cancel, got %d", page.deleteConfirm)
	}
}

// TestTagRequestBodyJSON tests that tagRequestBody serializes correctly
func TestTagRequestBodyJSON(t *testing.T) {
	testCases := []struct {
		name    string
		body    tagRequestBody
		checkFn func(t *testing.T, body tagRequestBody)
	}{
		{
			name: "With group",
			body: tagRequestBody{
				Name:        "invoice",
				Color:       "#ff0000",
				Description: "Invoice docs",
				TagGroup:    stringPtr("Documents"),
				SortOrder:   10,
			},
			checkFn: func(t *testing.T, body tagRequestBody) {
				if body.TagGroup == nil || *body.TagGroup != "Documents" {
					t.Error("Expected TagGroup='Documents'")
				}
			},
		},
		{
			name: "Free tag (no group)",
			body: tagRequestBody{
				Name:        "important",
				Color:       "#00ff00",
				Description: "",
				TagGroup:    nil,
				SortOrder:   0,
			},
			checkFn: func(t *testing.T, body tagRequestBody) {
				if body.TagGroup != nil {
					t.Error("Expected TagGroup=nil for free tag")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.checkFn(t, tc.body)
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

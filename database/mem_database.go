package database

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"codeberg.org/hum3/godocs/config"
	"github.com/oklog/ulid/v2"
)

// Compile-time check that *MemDB satisfies Repository.
var _ Repository = (*MemDB)(nil)

// MemDB is a pure in-memory Repository implementation for WASM demos and testing.
// It requires no external dependencies (no SQL driver, no CGo).
type MemDB struct {
	mu            sync.RWMutex
	docs          map[int]*Document       // id -> doc
	docsByULID    map[string]int          // ulid string -> id
	docsByPath    map[string]int          // path -> id
	docsByHash    map[string]int          // hash -> id
	tags          map[int]*Tag            // id -> tag
	tagAliases    []TagAliasEntry         // alias entries
	docTags       map[int]map[int]bool    // docID -> set of tagIDs
	stories       map[int]*Story          // id -> story
	storyTags     map[int]map[int]bool    // storyID -> set of tagIDs
	savedSearches map[int]*SavedSearch    // id -> saved search
	jobs          map[string]*Job         // ulid string -> job
	dimensions    map[int]*Dimension      // id -> dimension
	dimValues     map[int]*DimensionValue // id -> dimension value
	docDims       map[int]map[int]int     // docID -> dimensionID -> dimensionValueID
	cfg           *config.ServerConfig
	nextDocID     int
	nextTagID     int
	nextStoryID   int
	nextSearchID  int
	nextDimID     int
	nextDimValID  int
}

// NewMemDB creates a new in-memory repository.
func NewMemDB() *MemDB {
	return &MemDB{
		docs:          make(map[int]*Document),
		docsByULID:    make(map[string]int),
		docsByPath:    make(map[string]int),
		docsByHash:    make(map[string]int),
		tags:          make(map[int]*Tag),
		docTags:       make(map[int]map[int]bool),
		stories:       make(map[int]*Story),
		storyTags:     make(map[int]map[int]bool),
		savedSearches: make(map[int]*SavedSearch),
		jobs:          make(map[string]*Job),
		dimensions:    make(map[int]*Dimension),
		dimValues:     make(map[int]*DimensionValue),
		docDims:       make(map[int]map[int]int),
		nextDocID:     1,
		nextTagID:     1,
		nextStoryID:   1,
		nextSearchID:  1,
		nextDimID:     1,
		nextDimValID:  1,
	}
}

// Close is a no-op for the in-memory database.
func (m *MemDB) Close() error { return nil }

// ----------------------------------------------------------------------------
// Document methods
// ----------------------------------------------------------------------------

func (m *MemDB) SaveDocument(doc *Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ulidStr := doc.ULID.String()

	// Check if document with same path already exists (upsert behaviour)
	if existingID, ok := m.docsByPath[doc.Path]; ok {
		old := m.docs[existingID]
		// Remove old index entries
		delete(m.docsByULID, old.ULID.String())
		delete(m.docsByHash, old.Hash)
		// Update in place
		doc.ID = existingID
		cp := *doc
		m.docs[existingID] = &cp
		m.docsByULID[ulidStr] = existingID
		if doc.Hash != "" {
			m.docsByHash[doc.Hash] = existingID
		}
		return nil
	}

	if doc.ID == 0 {
		doc.ID = m.nextDocID
		m.nextDocID++
	}

	cp := *doc
	m.docs[doc.ID] = &cp
	m.docsByULID[ulidStr] = doc.ID
	m.docsByPath[doc.Path] = doc.ID
	if doc.Hash != "" {
		m.docsByHash[doc.Hash] = doc.ID
	}
	return nil
}

func (m *MemDB) GetDocumentByID(id int) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d, ok := m.docs[id]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (m *MemDB) GetDocumentByULID(ulidStr string) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id, ok := m.docsByULID[ulidStr]; ok {
		cp := *m.docs[id]
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (m *MemDB) GetDocumentByPath(path string) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id, ok := m.docsByPath[path]; ok {
		cp := *m.docs[id]
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (m *MemDB) GetDocumentByHash(hash string) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id, ok := m.docsByHash[hash]; ok {
		cp := *m.docs[id]
		return &cp, nil
	}
	// Match PGDB behaviour: return nil,nil for not found
	return nil, nil
}

// allDocsSorted returns all documents sorted by IngressTime descending.
// Caller must hold at least a read lock.
func (m *MemDB) allDocsSorted() []Document {
	docs := make([]Document, 0, len(m.docs))
	for _, d := range m.docs {
		docs = append(docs, *d)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].IngressTime.After(docs[j].IngressTime)
	})
	return docs
}

func (m *MemDB) GetNewestDocuments(limit int) ([]Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := m.allDocsSorted()
	if limit > len(all) {
		limit = len(all)
	}
	return all[:limit], nil
}

func (m *MemDB) GetNewestDocumentsWithPagination(page, pageSize int, showHidden ...bool) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := filterArchived(m.allDocsSorted())
	if shouldExcludeHidden(showHidden) {
		all = m.filterHidden(all)
	}

	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return []Document{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (m *MemDB) GetAllDocuments() ([]Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	docs := make([]Document, 0, len(m.docs))
	for _, d := range m.docs {
		docs = append(docs, *d)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	return docs, nil
}

func (m *MemDB) GetDocumentsByFolder(folder string) ([]Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Document
	for _, d := range m.docs {
		if d.Folder == folder {
			result = append(result, *d)
		}
	}
	return result, nil
}

func (m *MemDB) DeleteDocument(ulidStr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.docsByULID[ulidStr]
	if !ok {
		return nil
	}
	d := m.docs[id]
	delete(m.docsByULID, ulidStr)
	delete(m.docsByPath, d.Path)
	if d.Hash != "" {
		delete(m.docsByHash, d.Hash)
	}
	delete(m.docs, id)
	delete(m.docTags, id)
	delete(m.docDims, id)
	return nil
}

func (m *MemDB) UpdateDocumentURL(ulidStr string, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDoc(ulidStr, func(d *Document) { d.URL = url })
}

func (m *MemDB) UpdateDocumentPath(ulidStr string, path string, folder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDoc(ulidStr, func(d *Document) {
		delete(m.docsByPath, d.Path)
		d.Path = path
		d.Folder = folder
		m.docsByPath[path] = d.ID
	})
}

func (m *MemDB) UpdateDocumentFolder(ulidStr string, folder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDoc(ulidStr, func(d *Document) { d.Folder = folder })
}

func (m *MemDB) UpdateDocumentDate(ulidStr string, date *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDoc(ulidStr, func(d *Document) { d.DocumentDate = date })
}

func (m *MemDB) UpdateDocumentFullText(ulidStr string, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDoc(ulidStr, func(d *Document) { d.FullText = text })
}

func (m *MemDB) UpdateDocumentMetadata(ulidStr string, meta DocumentMetadataUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDoc(ulidStr, func(d *Document) {
		if meta.Name != nil {
			d.Name = *meta.Name
		}
		if meta.CreatedDate != nil {
			d.CreatedDate = meta.CreatedDate
		}
		if meta.UpdatedDate != nil {
			d.UpdatedDate = meta.UpdatedDate
		}
		if meta.Author != nil {
			d.Author = *meta.Author
		}
		if meta.SourceURL != nil {
			d.SourceURL = *meta.SourceURL
		}
		if meta.Source != nil {
			d.Source = *meta.Source
		}
	})
}

// updateDoc is a helper that applies fn to the document identified by ulidStr.
// Caller must hold m.mu write lock.
func (m *MemDB) updateDoc(ulidStr string, fn func(*Document)) error {
	id, ok := m.docsByULID[ulidStr]
	if !ok {
		return sql.ErrNoRows
	}
	fn(m.docs[id])
	return nil
}

func (m *MemDB) UpdateDocumentArchiveStatus(ulidStr string, status *string, archivedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateDoc(ulidStr, func(d *Document) {
		d.ArchiveStatus = status
		d.ArchivedAt = archivedAt
	})
}

func (m *MemDB) GetArchivedDocuments(page, pageSize int) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var archived []Document
	for _, d := range m.docs {
		if d.ArchiveStatus != nil {
			archived = append(archived, *d)
		}
	}
	total := len(archived)
	offset := (page - 1) * pageSize
	if offset >= total {
		return nil, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return archived[offset:end], total, nil
}

// ----------------------------------------------------------------------------
// Config
// ----------------------------------------------------------------------------

func (m *MemDB) SaveConfig(cfg *config.ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *cfg
	m.cfg = &cp
	return nil
}

func (m *MemDB) GetConfig() (*config.ServerConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg == nil {
		return nil, sql.ErrNoRows
	}
	cp := *m.cfg
	return &cp, nil
}

// ----------------------------------------------------------------------------
// Search
// ----------------------------------------------------------------------------

func (m *MemDB) SearchDocuments(searchTerm string) ([]Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	term := strings.ToLower(searchTerm)
	var result []Document
	for _, d := range m.docs {
		if strings.Contains(strings.ToLower(d.FullText), term) ||
			strings.Contains(strings.ToLower(d.Name), term) {
			result = append(result, *d)
		}
	}
	return result, nil
}

func (m *MemDB) ReindexSearchDocuments() (int, error) {
	return 0, nil // no-op for in-memory
}

// ----------------------------------------------------------------------------
// Word Cloud (stubs – return empty results)
// ----------------------------------------------------------------------------

func (m *MemDB) GetTopWords(limit int) ([]WordFrequency, error) {
	return []WordFrequency{}, nil
}

func (m *MemDB) GetWordCloudMetadata() (*WordCloudMetadata, error) {
	return &WordCloudMetadata{}, nil
}

func (m *MemDB) RecalculateAllWordFrequencies() error { return nil }

func (m *MemDB) UpdateWordFrequencies(docID string) error { return nil }

// ----------------------------------------------------------------------------
// Jobs
// ----------------------------------------------------------------------------

func (m *MemDB) CreateJob(jobType JobType, message string) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	entropy := ulid.Monotonic(rand.New(rand.NewSource(now.UnixNano())), 0)
	jobID, err := ulid.New(ulid.Timestamp(now), entropy)
	if err != nil {
		return nil, err
	}

	job := &Job{
		ID:        jobID,
		Type:      jobType,
		Status:    JobStatusPending,
		Message:   message,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.jobs[jobID.String()] = job
	return job, nil
}

func (m *MemDB) UpdateJobProgress(jobID ulid.ULID, progress int, currentStep string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID.String()]
	if !ok {
		return sql.ErrNoRows
	}
	j.Progress = progress
	j.CurrentStep = currentStep
	j.UpdatedAt = time.Now()
	return nil
}

func (m *MemDB) UpdateJobStatus(jobID ulid.ULID, status JobStatus, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID.String()]
	if !ok {
		return sql.ErrNoRows
	}
	now := time.Now()
	j.Status = status
	j.Message = message
	j.UpdatedAt = now
	if status == JobStatusRunning && j.StartedAt == nil {
		j.StartedAt = &now
	}
	if status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled {
		j.CompletedAt = &now
	}
	return nil
}

func (m *MemDB) UpdateJobError(jobID ulid.ULID, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID.String()]
	if !ok {
		return sql.ErrNoRows
	}
	now := time.Now()
	j.Status = JobStatusFailed
	j.Error = errorMsg
	j.UpdatedAt = now
	j.CompletedAt = &now
	return nil
}

func (m *MemDB) CompleteJob(jobID ulid.ULID, result string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID.String()]
	if !ok {
		return sql.ErrNoRows
	}
	now := time.Now()
	j.Status = JobStatusCompleted
	j.Progress = 100
	j.Result = result
	j.UpdatedAt = now
	j.CompletedAt = &now
	return nil
}

func (m *MemDB) GetJob(jobID ulid.ULID) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[jobID.String()]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *j
	return &cp, nil
}

func (m *MemDB) GetRecentJobs(limit, offset int) ([]Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		all = append(all, *j)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	if offset >= len(all) {
		return []Job{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (m *MemDB) GetActiveJobs() ([]Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Job
	for _, j := range m.jobs {
		if j.Status == JobStatusPending || j.Status == JobStatusRunning {
			result = append(result, *j)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (m *MemDB) DeleteOldJobs(olderThan time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-olderThan)
	count := 0
	for k, j := range m.jobs {
		if (j.Status == JobStatusCompleted || j.Status == JobStatusFailed || j.Status == JobStatusCancelled) &&
			j.CompletedAt != nil && j.CompletedAt.Before(cutoff) {
			delete(m.jobs, k)
			count++
		}
	}
	return count, nil
}

func (m *MemDB) CancelJob(jobID ulid.ULID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, j := range m.jobs {
		if j.ID == jobID && (j.Status == JobStatusPending || j.Status == JobStatusRunning) {
			now := time.Now()
			m.jobs[k].Status = JobStatusCancelled
			m.jobs[k].Message = "Cancelled by user"
			m.jobs[k].UpdatedAt = now
			m.jobs[k].CompletedAt = &now
			return nil
		}
	}
	return fmt.Errorf("job %s not found or already completed", jobID)
}

func (m *MemDB) RecoverStuckJobs(stuckThreshold time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-stuckThreshold)
	now := time.Now()
	count := 0
	for k, j := range m.jobs {
		if (j.Status == JobStatusPending || j.Status == JobStatusRunning) && j.UpdatedAt.Before(cutoff) {
			m.jobs[k].Status = JobStatusFailed
			m.jobs[k].Error = "Recovered: exceeded timeout"
			m.jobs[k].UpdatedAt = now
			m.jobs[k].CompletedAt = &now
			count++
		}
	}
	return count, nil
}

// ----------------------------------------------------------------------------
// Tags
// ----------------------------------------------------------------------------

func (m *MemDB) CreateTag(tag *Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	tag.ID = m.nextTagID
	m.nextTagID++
	tag.CreatedAt = now
	tag.UpdatedAt = now
	cp := *tag
	m.tags[tag.ID] = &cp
	return nil
}

func (m *MemDB) GetAllTags() ([]Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tags := make([]Tag, 0, len(m.tags))
	for _, t := range m.tags {
		tags = append(tags, *t)
	}
	sort.Slice(tags, func(i, j int) bool {
		gi, gj := "", ""
		if tags[i].TagGroup != nil {
			gi = *tags[i].TagGroup
		}
		if tags[j].TagGroup != nil {
			gj = *tags[j].TagGroup
		}
		// nil groups first
		if (gi == "") != (gj == "") {
			return gi == ""
		}
		if gi != gj {
			return gi < gj
		}
		if tags[i].SortOrder != tags[j].SortOrder {
			return tags[i].SortOrder < tags[j].SortOrder
		}
		return tags[i].Name < tags[j].Name
	})
	return tags, nil
}

func (m *MemDB) GetTagByID(id int) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if t, ok := m.tags[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, nil
}

func (m *MemDB) GetTagByName(name string) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tags {
		if t.Name == name {
			cp := *t
			return &cp, nil
		}
	}
	// Check aliases
	for _, a := range m.tagAliases {
		if a.AliasName == name {
			for _, t := range m.tags {
				if t.Name == a.TagName {
					cp := *t
					return &cp, nil
				}
			}
		}
	}
	return nil, nil
}

func (m *MemDB) UpdateTag(tag *Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.tags[tag.ID]
	if !ok {
		return sql.ErrNoRows
	}
	// If renamed, save old name as alias
	if old.Name != tag.Name {
		m.tagAliases = append(m.tagAliases, TagAliasEntry{AliasName: old.Name, TagName: tag.Name})
	}
	tag.UpdatedAt = time.Now()
	cp := *tag
	m.tags[tag.ID] = &cp
	return nil
}

func (m *MemDB) DeleteTag(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tags, id)
	// Remove from all document-tag associations
	for docID, tagSet := range m.docTags {
		delete(tagSet, id)
		if len(tagSet) == 0 {
			delete(m.docTags, docID)
		}
	}
	// Remove from story tags
	for storyID, tagSet := range m.storyTags {
		delete(tagSet, id)
		if len(tagSet) == 0 {
			delete(m.storyTags, storyID)
		}
	}
	return nil
}

func (m *MemDB) GetTagsForDocument(documentID int) ([]Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tagIDs := m.docTags[documentID]
	tags := make([]Tag, 0, len(tagIDs))
	for tid := range tagIDs {
		if t, ok := m.tags[tid]; ok {
			tags = append(tags, *t)
		}
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return tags, nil
}

func (m *MemDB) AddTagToDocument(documentID int, tagID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.docTags[documentID] == nil {
		m.docTags[documentID] = make(map[int]bool)
	}
	m.docTags[documentID][tagID] = true
	return nil
}

func (m *MemDB) RemoveTagFromDocument(documentID int, tagID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.docTags[documentID]; s != nil {
		delete(s, tagID)
	}
	return nil
}

func (m *MemDB) GetTagUsageCount(tagID int) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, tagSet := range m.docTags {
		if tagSet[tagID] {
			count++
		}
	}
	return count, nil
}

func (m *MemDB) GetTopTagsByUsage(limit int) ([]TagWithCount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Count usage per tag
	counts := make(map[int]int)
	for _, tagSet := range m.docTags {
		for tid := range tagSet {
			counts[tid]++
		}
	}

	// Build TagWithCount slice for tags that have at least one document
	var result []TagWithCount
	for tid, count := range counts {
		if t, ok := m.tags[tid]; ok {
			result = append(result, TagWithCount{Tag: *t, DocumentCount: count})
		}
	}

	// Sort by count desc, then name asc
	sort.Slice(result, func(i, j int) bool {
		if result[i].DocumentCount != result[j].DocumentCount {
			return result[i].DocumentCount > result[j].DocumentCount
		}
		return result[i].Name < result[j].Name
	})

	if limit > len(result) {
		limit = len(result)
	}
	return result[:limit], nil
}

func (m *MemDB) GetTagGroups() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]bool{}
	for _, t := range m.tags {
		if t.TagGroup != nil && *t.TagGroup != "" {
			seen[*t.TagGroup] = true
		}
	}
	groups := make([]string, 0, len(seen))
	for g := range seen {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	return groups, nil
}

func (m *MemDB) GetAllTagAliases() ([]TagAliasEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make([]TagAliasEntry, len(m.tagAliases))
	copy(cp, m.tagAliases)
	return cp, nil
}

func (m *MemDB) InsertTagAlias(tagID int, aliasName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Check for duplicate alias name
	for _, a := range m.tagAliases {
		if a.AliasName == aliasName {
			return nil // ON CONFLICT DO NOTHING equivalent
		}
	}
	tagName := ""
	if t, ok := m.tags[tagID]; ok {
		tagName = t.Name
	}
	m.tagAliases = append(m.tagAliases, TagAliasEntry{AliasName: aliasName, TagName: tagName})
	return nil
}

// ----------------------------------------------------------------------------
// Dimensions
// ----------------------------------------------------------------------------

func (m *MemDB) GetAllDimensions() ([]Dimension, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dims := make([]Dimension, 0, len(m.dimensions))
	for _, d := range m.dimensions {
		dims = append(dims, *d)
	}
	sort.Slice(dims, func(i, j int) bool { return dims[i].Name < dims[j].Name })
	return dims, nil
}

func (m *MemDB) GetDimensionByID(id int) (*Dimension, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d, ok := m.dimensions[id]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, nil
}

func (m *MemDB) GetDimensionByName(name string) (*Dimension, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.dimensions {
		if d.Name == name {
			cp := *d
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *MemDB) GetDimensionValues(dimensionID int) ([]DimensionValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var vals []DimensionValue
	for _, v := range m.dimValues {
		if v.DimensionID == dimensionID {
			vals = append(vals, *v)
		}
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i].SortOrder < vals[j].SortOrder })
	return vals, nil
}

func (m *MemDB) GetDimensionValueByValue(dimensionID int, value string) (*DimensionValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.dimValues {
		if v.DimensionID == dimensionID && v.Value == value {
			cp := *v
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *MemDB) GetDocumentDimensions(documentID int) (map[string]DimensionValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]DimensionValue)
	dimMap := m.docDims[documentID]
	for dimID, dvID := range dimMap {
		dim, ok := m.dimensions[dimID]
		if !ok {
			continue
		}
		dv, ok := m.dimValues[dvID]
		if !ok {
			continue
		}
		result[dim.Name] = *dv
	}
	return result, nil
}

func (m *MemDB) SetDocumentDimension(documentID int, dimensionID int, dimensionValueID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.docDims[documentID] == nil {
		m.docDims[documentID] = make(map[int]int)
	}
	m.docDims[documentID][dimensionID] = dimensionValueID
	return nil
}

func (m *MemDB) RemoveDocumentDimension(documentID int, dimensionID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if dm := m.docDims[documentID]; dm != nil {
		delete(dm, dimensionID)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Schema version
// ----------------------------------------------------------------------------

func (m *MemDB) GetSchemaVersion() (string, error) {
	return "mem-1.0", nil
}

// ----------------------------------------------------------------------------
// Saved Searches
// ----------------------------------------------------------------------------

func (m *MemDB) GetAllSavedSearches() ([]SavedSearch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]SavedSearch, 0, len(m.savedSearches))
	for _, s := range m.savedSearches {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SortOrder != result[j].SortOrder {
			return result[i].SortOrder < result[j].SortOrder
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (m *MemDB) GetSavedSearchByID(id int) (*SavedSearch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.savedSearches[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (m *MemDB) CreateSavedSearch(search *SavedSearch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	search.ID = m.nextSearchID
	m.nextSearchID++
	cp := *search
	m.savedSearches[search.ID] = &cp
	return nil
}

func (m *MemDB) UpdateSavedSearch(search *SavedSearch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.savedSearches[search.ID]; !ok {
		return sql.ErrNoRows
	}
	cp := *search
	m.savedSearches[search.ID] = &cp
	return nil
}

func (m *MemDB) DeleteSavedSearch(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.savedSearches, id)
	return nil
}

// ----------------------------------------------------------------------------
// Search execution
// ----------------------------------------------------------------------------

// docHasTag returns true if the document has a tag whose name matches (case-insensitive).
// Caller must hold at least a read lock.
func (m *MemDB) docHasTag(docID int, tagName string) bool {
	for tid := range m.docTags[docID] {
		if t, ok := m.tags[tid]; ok && strings.EqualFold(t.Name, tagName) {
			return true
		}
	}
	return false
}

// docHasAnyTag returns true if the document has at least one tag.
func (m *MemDB) docHasAnyTag(docID int) bool {
	return len(m.docTags[docID]) > 0
}

// filterHidden removes documents tagged "Hide" from a slice.
// Caller must hold at least a read lock.
func (m *MemDB) filterHidden(docs []Document) []Document {
	var out []Document
	for _, d := range docs {
		if !m.docHasTag(d.ID, "Hide") {
			out = append(out, d)
		}
	}
	return out
}

// filterArchived removes documents with a non-nil ArchiveStatus from a slice.
func filterArchived(docs []Document) []Document {
	var out []Document
	for _, d := range docs {
		if d.ArchiveStatus == nil {
			out = append(out, d)
		}
	}
	return out
}

func (m *MemDB) GetDocumentsByTag(tagID int, page, pageSize int, showHidden ...bool) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var docs []Document
	for _, d := range filterArchived(m.allDocsSorted()) {
		if m.docTags[d.ID][tagID] {
			docs = append(docs, d)
		}
	}
	if shouldExcludeHidden(showHidden) {
		docs = m.filterHidden(docs)
	}
	return paginate(docs, page, pageSize)
}

func (m *MemDB) GetUntaggedDocuments(page, pageSize int, showHidden ...bool) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var docs []Document
	for _, d := range filterArchived(m.allDocsSorted()) {
		if !m.docHasAnyTag(d.ID) {
			docs = append(docs, d)
		}
	}
	if shouldExcludeHidden(showHidden) {
		docs = m.filterHidden(docs)
	}
	return paginate(docs, page, pageSize)
}

func (m *MemDB) GetTaggedDocuments(page, pageSize int, showHidden ...bool) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var docs []Document
	for _, d := range filterArchived(m.allDocsSorted()) {
		if m.docHasAnyTag(d.ID) {
			docs = append(docs, d)
		}
	}
	if shouldExcludeHidden(showHidden) {
		docs = m.filterHidden(docs)
	}
	return paginate(docs, page, pageSize)
}

func (m *MemDB) ExecuteSearch(parsed *ParsedSearch, page, pageSize int, showHidden ...bool) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if parsed.IsAllDocs {
		// Delegate but we already hold the read lock – release and call the public method.
		m.mu.RUnlock()
		defer m.mu.RLock()
		return m.GetNewestDocumentsWithPagination(page, pageSize, showHidden...)
	}
	if parsed.IsUntagged {
		m.mu.RUnlock()
		defer m.mu.RLock()
		return m.GetUntaggedDocuments(page, pageSize, showHidden...)
	}
	if parsed.IsTagged {
		m.mu.RUnlock()
		defer m.mu.RLock()
		return m.GetTaggedDocuments(page, pageSize, showHidden...)
	}

	all := filterArchived(m.allDocsSorted())
	var filtered []Document

	for _, d := range all {
		match := true

		// Include tags: doc must have ALL
		for _, tn := range parsed.IncludeTags {
			if !m.docHasTag(d.ID, tn) {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		// Exclude tags: doc must NOT have any
		for _, tn := range parsed.ExcludeTags {
			if m.docHasTag(d.ID, tn) {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		// Text search: every term must appear in name or full_text (case-insensitive)
		if parsed.TextTerms != "" {
			terms := strings.Fields(parsed.TextTerms)
			nameLower := strings.ToLower(d.Name)
			textLower := strings.ToLower(d.FullText)
			for _, term := range terms {
				tl := strings.ToLower(term)
				if !strings.Contains(nameLower, tl) && !strings.Contains(textLower, tl) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// Date range filters
		if parsed.AfterDate != nil && (d.DocumentDate == nil || d.DocumentDate.Before(*parsed.AfterDate)) {
			continue
		}
		if parsed.BeforeDate != nil && (d.DocumentDate == nil || d.DocumentDate.After(*parsed.BeforeDate)) {
			continue
		}

		filtered = append(filtered, d)
	}

	// Hide exclusion
	if shouldExcludeHidden(showHidden) {
		searchingForHide := false
		for _, tag := range parsed.IncludeTags {
			if strings.EqualFold(tag, "Hide") {
				searchingForHide = true
				break
			}
		}
		if !searchingForHide {
			filtered = m.filterHidden(filtered)
		}
	}

	return paginate(filtered, page, pageSize)
}

// ----------------------------------------------------------------------------
// Stories
// ----------------------------------------------------------------------------

func (m *MemDB) CreateStory(story *Story) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Create the story tag
	tagName := storyTagName(story.Title)
	storyGroup := "Story"
	tag := &Tag{
		ID:        m.nextTagID,
		Name:      tagName,
		Color:     "#8e44ad",
		TagGroup:  &storyGroup,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.nextTagID++
	m.tags[tag.ID] = tag

	story.TagID = tag.ID
	story.ID = m.nextStoryID
	m.nextStoryID++
	story.CreatedAt = now
	story.UpdatedAt = now

	cp := *story
	m.stories[story.ID] = &cp
	return nil
}

func (m *MemDB) GetStoryByID(id int) (*Story, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.stories[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (m *MemDB) GetAllStories() ([]Story, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Story, 0, len(m.stories))
	for _, s := range m.stories {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		si, sj := result[i].StartDate, result[j].StartDate
		// Non-nil dates first, then descending
		if (si == nil) != (sj == nil) {
			return si != nil
		}
		if si != nil && sj != nil && !si.Equal(*sj) {
			return si.After(*sj)
		}
		return result[i].Title < result[j].Title
	})
	return result, nil
}

func (m *MemDB) GetStoriesWithMeta() ([]StoryWithMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stories := make([]Story, 0, len(m.stories))
	for _, s := range m.stories {
		stories = append(stories, *s)
	}
	sort.Slice(stories, func(i, j int) bool {
		si, sj := stories[i].StartDate, stories[j].StartDate
		if (si == nil) != (sj == nil) {
			return si != nil
		}
		if si != nil && sj != nil && !si.Equal(*sj) {
			return si.After(*sj)
		}
		return stories[i].Title < stories[j].Title
	})

	result := make([]StoryWithMeta, len(stories))
	for i, s := range stories {
		result[i].Story = s
		if s.StartDate != nil {
			result[i].StartDateFmt = s.StartDate.Format("2006-01-02")
		}
		if s.EndDate != nil {
			result[i].EndDateFmt = s.EndDate.Format("2006-01-02")
		}

		// Story's own tag
		if t, ok := m.tags[s.TagID]; ok {
			result[i].Tag = *t
		} else {
			result[i].Tag = Tag{ID: s.TagID, Name: storyTagName(s.Title)}
		}

		// Associated tags
		assocTags := make([]Tag, 0)
		for tid := range m.storyTags[s.ID] {
			if t, ok := m.tags[tid]; ok {
				assocTags = append(assocTags, *t)
			}
		}
		sort.Slice(assocTags, func(a, b int) bool { return assocTags[a].Name < assocTags[b].Name })
		result[i].AssociatedTags = assocTags

		// Document count: docs with this story's tag
		count := 0
		for _, tagSet := range m.docTags {
			if tagSet[s.TagID] {
				count++
			}
		}
		result[i].DocumentCount = count
	}
	return result, nil
}

func (m *MemDB) UpdateStory(story *Story) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.stories[story.ID]
	if !ok {
		return sql.ErrNoRows
	}
	now := time.Now()
	story.UpdatedAt = now
	cp := *story
	m.stories[story.ID] = &cp

	// Sync tag name
	if t, ok2 := m.tags[old.TagID]; ok2 {
		t.Name = storyTagName(story.Title)
		t.Description = story.Title
		t.UpdatedAt = now
	}
	return nil
}

func (m *MemDB) DeleteStory(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.stories[id]
	if !ok {
		return nil
	}
	tagID := s.TagID
	delete(m.stories, id)
	delete(m.storyTags, id)
	// Delete the tag and cascade doc associations
	delete(m.tags, tagID)
	for docID, tagSet := range m.docTags {
		delete(tagSet, tagID)
		if len(tagSet) == 0 {
			delete(m.docTags, docID)
		}
	}
	return nil
}

func (m *MemDB) GetStoryByTagID(tagID int) (*Story, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.stories {
		if s.TagID == tagID {
			cp := *s
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *MemDB) GetStoryTags(storyID int) ([]Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tagIDs := m.storyTags[storyID]
	tags := make([]Tag, 0, len(tagIDs))
	for tid := range tagIDs {
		if t, ok := m.tags[tid]; ok {
			tags = append(tags, *t)
		}
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return tags, nil
}

func (m *MemDB) AddStoryTag(storyID int, tagID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storyTags[storyID] == nil {
		m.storyTags[storyID] = make(map[int]bool)
	}
	m.storyTags[storyID][tagID] = true
	return nil
}

func (m *MemDB) RemoveStoryTag(storyID int, tagID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.storyTags[storyID]; s != nil {
		delete(s, tagID)
	}
	return nil
}

func (m *MemDB) AddDocumentToStory(documentID int, storyID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.stories[storyID]
	if !ok {
		return fmt.Errorf("story not found: %d", storyID)
	}
	// Add story's own tag
	if m.docTags[documentID] == nil {
		m.docTags[documentID] = make(map[int]bool)
	}
	m.docTags[documentID][s.TagID] = true
	// Add associated tags
	for tid := range m.storyTags[storyID] {
		m.docTags[documentID][tid] = true
	}
	return nil
}

func (m *MemDB) RemoveDocumentFromStory(documentID int, storyID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.stories[storyID]
	if !ok {
		return fmt.Errorf("story not found: %d", storyID)
	}
	if tagSet := m.docTags[documentID]; tagSet != nil {
		delete(tagSet, s.TagID)
	}
	return nil
}

func (m *MemDB) GetDocumentsWithoutStory(page, pageSize int, showHidden ...bool) ([]Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all tag IDs that belong to group "Story"
	storyTagIDs := make(map[int]bool)
	for _, t := range m.tags {
		if t.TagGroup != nil && *t.TagGroup == "Story" {
			storyTagIDs[t.ID] = true
		}
	}

	all := filterArchived(m.allDocsSorted())
	var docs []Document
	for _, d := range all {
		hasStory := false
		for tid := range m.docTags[d.ID] {
			if storyTagIDs[tid] {
				hasStory = true
				break
			}
		}
		if !hasStory {
			docs = append(docs, d)
		}
	}

	if shouldExcludeHidden(showHidden) {
		docs = m.filterHidden(docs)
	}
	return paginate(docs, page, pageSize)
}

func (m *MemDB) ConvertTagToStory(tagID int) (*Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[tagID]
	if !ok {
		return nil, fmt.Errorf("tag not found: %d", tagID)
	}

	now := time.Now()
	storyGroup := "Story"
	tag.TagGroup = &storyGroup
	tag.UpdatedAt = now

	story := &Story{
		ID:        m.nextStoryID,
		Title:     strings.ReplaceAll(tag.Name, "-", " "),
		TagID:     tagID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.nextStoryID++
	cp := *story
	m.stories[story.ID] = &cp

	return story, nil
}

func (m *MemDB) ConvertStoryToTag(storyID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.stories[storyID]
	if !ok {
		return fmt.Errorf("story not found: %d", storyID)
	}

	// Clear tag group
	if t, ok2 := m.tags[s.TagID]; ok2 {
		t.TagGroup = nil
		t.UpdatedAt = time.Now()
	}

	// Delete story row (keep tag)
	delete(m.stories, storyID)
	delete(m.storyTags, storyID)
	return nil
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// paginate returns a page slice and the total count.
func paginate(docs []Document, page, pageSize int) ([]Document, int, error) {
	total := len(docs)
	start := (page - 1) * pageSize
	if start >= total {
		return []Document{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return docs[start:end], total, nil
}

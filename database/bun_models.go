package database

import (
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

// BunDocument represents the documents table for Bun ORM
type BunDocument struct {
	bun.BaseModel `bun:"table:documents,alias:d"`

	ID             int       `bun:"id,pk,autoincrement"`
	Name           string    `bun:"name,notnull"`
	Path           string    `bun:"path,notnull,unique"`
	IngressTime    time.Time `bun:"ingress_time,notnull,default:current_timestamp"`
	Folder         string    `bun:"folder,notnull"`
	Hash           string    `bun:"hash,notnull"`
	ULID           string    `bun:"ulid,notnull,unique"` // Stored as string in DB
	DocumentType   string    `bun:"document_type,notnull"`
	FullText       string    `bun:"full_text,nullzero"`
	URL            string    `bun:"url,nullzero"`
	FullTextSearch string    `bun:"full_text_search,type:tsvector,nullzero"` // PostgreSQL-specific
	CreatedAt      time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt      time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

// ToDocument converts BunDocument to Document
func (bd *BunDocument) ToDocument() (*Document, error) {
	parsedULID, err := ulid.Parse(bd.ULID)
	if err != nil {
		return nil, err
	}

	return &Document{
		ID:           bd.ID,
		Name:         bd.Name,
		Path:         bd.Path,
		IngressTime:  bd.IngressTime,
		Folder:       bd.Folder,
		Hash:         bd.Hash,
		ULID:         parsedULID,
		DocumentType: bd.DocumentType,
		FullText:     bd.FullText,
		URL:          bd.URL,
	}, nil
}

// FromDocument converts Document to BunDocument
func FromDocument(doc *Document) *BunDocument {
	return &BunDocument{
		ID:           doc.ID,
		Name:         doc.Name,
		Path:         doc.Path,
		IngressTime:  doc.IngressTime,
		Folder:       doc.Folder,
		Hash:         doc.Hash,
		ULID:         doc.ULID.String(),
		DocumentType: doc.DocumentType,
		FullText:     doc.FullText,
		URL:          doc.URL,
	}
}

// BunServerConfig represents the server_config table for Bun ORM
type BunServerConfig struct {
	bun.BaseModel `bun:"table:server_config,alias:sc"`

	ID                   int       `bun:"id,pk"`
	ListenAddrIP         string    `bun:"listen_addr_ip,default:''"`
	ListenAddrPort       string    `bun:"listen_addr_port,notnull,default:'8000'"`
	IngressPath          string    `bun:"ingress_path,notnull,default:''"`
	IngressDelete        bool      `bun:"ingress_delete,notnull,default:false"`
	IngressMoveFolder    string    `bun:"ingress_move_folder,notnull,default:''"`
	IngressPreserve      bool      `bun:"ingress_preserve,notnull,default:true"`
	DocumentPath         string    `bun:"document_path,notnull,default:''"`
	NewDocumentFolder    string    `bun:"new_document_folder,default:''"`
	NewDocumentFolderRel string    `bun:"new_document_folder_rel,default:''"`
	WebUIPass            bool      `bun:"web_ui_pass,notnull,default:false"`
	ClientUsername       string    `bun:"client_username,default:''"`
	ClientPassword       string    `bun:"client_password,default:''"`
	PushBulletToken      string    `bun:"pushbullet_token,default:''"`
	TesseractPath        string    `bun:"tesseract_path,default:''"`
	UseReverseProxy      bool      `bun:"use_reverse_proxy,notnull,default:false"`
	BaseURL              string    `bun:"base_url,default:''"`
	IngressInterval      int       `bun:"ingress_interval,notnull,default:10"`
	NewDocumentNumber    int       `bun:"new_document_number,notnull,default:5"`
	ServerAPIURL         string    `bun:"server_api_url,default:''"`
	CreatedAt            time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt            time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

// BunJob represents the jobs table for Bun ORM
type BunJob struct {
	bun.BaseModel `bun:"table:jobs,alias:j"`

	ID          string     `bun:"id,pk"` // ULID as string
	Type        string     `bun:"type,notnull"`
	Status      string     `bun:"status,default:'pending'"`
	Progress    int        `bun:"progress,default:0"`
	CurrentStep string     `bun:"current_step,default:''"`
	TotalSteps  int        `bun:"total_steps,default:0"`
	Message     string     `bun:"message,default:''"`
	Error       string     `bun:"error,nullzero"`
	Result      string     `bun:"result,nullzero"`
	CreatedAt   time.Time  `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt   time.Time  `bun:"updated_at,notnull,default:current_timestamp"`
	StartedAt   *time.Time `bun:"started_at,nullzero"`
	CompletedAt *time.Time `bun:"completed_at,nullzero"`
}

// ToJob converts BunJob to Job
func (bj *BunJob) ToJob() (*Job, error) {
	parsedULID, err := ulid.Parse(bj.ID)
	if err != nil {
		return nil, err
	}

	return &Job{
		ID:          parsedULID,
		Type:        JobType(bj.Type),
		Status:      JobStatus(bj.Status),
		Progress:    bj.Progress,
		CurrentStep: bj.CurrentStep,
		TotalSteps:  bj.TotalSteps,
		Message:     bj.Message,
		Error:       bj.Error,
		Result:      bj.Result,
		CreatedAt:   bj.CreatedAt,
		UpdatedAt:   bj.UpdatedAt,
		StartedAt:   bj.StartedAt,
		CompletedAt: bj.CompletedAt,
	}, nil
}

// FromJob converts Job to BunJob
func FromJob(job *Job) *BunJob {
	return &BunJob{
		ID:          job.ID.String(),
		Type:        string(job.Type),
		Status:      string(job.Status),
		Progress:    job.Progress,
		CurrentStep: job.CurrentStep,
		TotalSteps:  job.TotalSteps,
		Message:     job.Message,
		Error:       job.Error,
		Result:      job.Result,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
	}
}

// BunWordFrequency represents the word_frequencies table for Bun ORM
type BunWordFrequency struct {
	bun.BaseModel `bun:"table:word_frequencies,alias:wf"`

	Word        string    `bun:"word,pk"`
	Frequency   int       `bun:"frequency,default:1"`
	LastUpdated time.Time `bun:"last_updated,default:current_timestamp"`
}

// ToWordFrequency converts BunWordFrequency to WordFrequency
func (bwf *BunWordFrequency) ToWordFrequency() *WordFrequency {
	return &WordFrequency{
		Word:      bwf.Word,
		Frequency: bwf.Frequency,
		Updated:   bwf.LastUpdated,
	}
}

// BunWordCloudMetadata represents the word_cloud_metadata table for Bun ORM
type BunWordCloudMetadata struct {
	bun.BaseModel `bun:"table:word_cloud_metadata,alias:wcm"`

	ID                  int        `bun:"id,pk"`
	LastFullCalculation *time.Time `bun:"last_full_calculation,nullzero"`
	TotalDocsProcessed  int        `bun:"total_documents_processed,default:0"`
	TotalWordsIndexed   int        `bun:"total_words_indexed,default:0"`
	Version             int        `bun:"version,default:1"`
	CreatedAt           time.Time  `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt           time.Time  `bun:"updated_at,notnull,default:current_timestamp"`
}

// ToWordCloudMetadata converts BunWordCloudMetadata to WordCloudMetadata
func (bwcm *BunWordCloudMetadata) ToWordCloudMetadata() *WordCloudMetadata {
	meta := &WordCloudMetadata{
		TotalDocsProcessed: bwcm.TotalDocsProcessed,
		TotalWordsIndexed:  bwcm.TotalWordsIndexed,
		Version:            bwcm.Version,
	}

	if bwcm.LastFullCalculation != nil {
		meta.LastCalculation = *bwcm.LastFullCalculation
	}

	return meta
}

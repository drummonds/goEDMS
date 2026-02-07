package database

import (
	"time"

	"github.com/oklog/ulid/v2"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// JobType represents the type of job
type JobType string

const (
	JobTypeIngestion     JobType = "ingestion"
	JobTypeCleanup       JobType = "cleanup"
	JobTypeWordCloud     JobType = "wordcloud"
	JobTypeSearchReindex JobType = "search_reindex"
)

// Job represents a background job or operation
type Job struct {
	ID          ulid.ULID  `json:"id"`
	Type        JobType    `json:"type"`
	Status      JobStatus  `json:"status"`
	Progress    int        `json:"progress"`         // 0-100
	CurrentStep string     `json:"currentStep"`      // Human-readable current step
	TotalSteps  int        `json:"totalSteps"`       // Total number of steps
	Message     string     `json:"message"`          // Status message
	Error       string     `json:"error,omitempty"`  // Error message if failed
	Result      string     `json:"result,omitempty"` // JSON result data
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// JobSummary provides summary statistics for a job
type JobSummary struct {
	FilesProcessed int    `json:"filesProcessed"`
	FilesTotal     int    `json:"filesTotal"`
	BytesProcessed int64  `json:"bytesProcessed"`
	Errors         int    `json:"errors"`
	Details        string `json:"details,omitempty"`
}

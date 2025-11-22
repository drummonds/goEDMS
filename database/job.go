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

// CreateJob creates a new job in the database
func (p *PostgresDB) CreateJob(jobType JobType, message string) (*Job, error) {
	now := time.Now()
	jobID, err := CalculateUUID(now)
	if err != nil {
		return nil, err
	}

	job := &Job{
		ID:          jobID,
		Type:        jobType,
		Status:      JobStatusPending,
		Progress:    0,
		CurrentStep: "",
		TotalSteps:  0,
		Message:     message,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	query := `
		INSERT INTO jobs (id, type, status, progress, current_step, total_steps, message, error, result, created_at, updated_at, started_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = p.db.Exec(query,
		job.ID.String(),
		job.Type,
		job.Status,
		job.Progress,
		job.CurrentStep,
		job.TotalSteps,
		job.Message,
		job.Error,
		job.Result,
		job.CreatedAt,
		job.UpdatedAt,
		job.StartedAt,
		job.CompletedAt,
	)

	if err != nil {
		return nil, err
	}

	return job, nil
}

// UpdateJobProgress updates the progress of a job
func (p *PostgresDB) UpdateJobProgress(jobID ulid.ULID, progress int, currentStep string) error {
	query := `
		UPDATE jobs
		SET progress = $1, current_step = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := p.db.Exec(query, progress, currentStep, time.Now(), jobID.String())
	return err
}

// UpdateJobStatus updates the status of a job
func (p *PostgresDB) UpdateJobStatus(jobID ulid.ULID, status JobStatus, message string) error {
	now := time.Now()
	var startedAt, completedAt interface{}

	if status == JobStatusRunning {
		startedAt = now
	}
	if status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled {
		completedAt = now
	}

	query := `
		UPDATE jobs
		SET status = $1, message = $2, updated_at = $3, started_at = COALESCE(started_at, $4), completed_at = $5
		WHERE id = $6
	`
	_, err := p.db.Exec(query, status, message, now, startedAt, completedAt, jobID.String())
	return err
}

// UpdateJobError updates a job with an error
func (p *PostgresDB) UpdateJobError(jobID ulid.ULID, errorMsg string) error {
	now := time.Now()
	query := `
		UPDATE jobs
		SET status = $1, error = $2, updated_at = $3, completed_at = $4
		WHERE id = $5
	`
	_, err := p.db.Exec(query, JobStatusFailed, errorMsg, now, now, jobID.String())
	return err
}

// CompleteJob marks a job as completed with optional result data
func (p *PostgresDB) CompleteJob(jobID ulid.ULID, result string) error {
	now := time.Now()
	query := `
		UPDATE jobs
		SET status = $1, progress = 100, result = $2, updated_at = $3, completed_at = $4
		WHERE id = $5
	`
	_, err := p.db.Exec(query, JobStatusCompleted, result, now, now, jobID.String())
	return err
}

// GetJob retrieves a job by ID
func (p *PostgresDB) GetJob(jobID ulid.ULID) (*Job, error) {
	query := `
		SELECT id, type, status, progress, current_step, total_steps, message, error, result,
		       created_at, updated_at, started_at, completed_at
		FROM jobs
		WHERE id = $1
	`

	job := &Job{}
	var idStr string

	err := p.db.QueryRow(query, jobID.String()).Scan(
		&idStr,
		&job.Type,
		&job.Status,
		&job.Progress,
		&job.CurrentStep,
		&job.TotalSteps,
		&job.Message,
		&job.Error,
		&job.Result,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.StartedAt,
		&job.CompletedAt,
	)

	if err != nil {
		return nil, err
	}

	job.ID, err = ulid.Parse(idStr)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// GetRecentJobs retrieves the most recent jobs with pagination
func (p *PostgresDB) GetRecentJobs(limit, offset int) ([]Job, error) {
	query := `
		SELECT id, type, status, progress, current_step, total_steps, message, error, result,
		       created_at, updated_at, started_at, completed_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := p.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var idStr string

		err := rows.Scan(
			&idStr,
			&job.Type,
			&job.Status,
			&job.Progress,
			&job.CurrentStep,
			&job.TotalSteps,
			&job.Message,
			&job.Error,
			&job.Result,
			&job.CreatedAt,
			&job.UpdatedAt,
			&job.StartedAt,
			&job.CompletedAt,
		)

		if err != nil {
			return nil, err
		}

		job.ID, err = ulid.Parse(idStr)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetActiveJobs retrieves all running or pending jobs
func (p *PostgresDB) GetActiveJobs() ([]Job, error) {
	query := `
		SELECT id, type, status, progress, current_step, total_steps, message, error, result,
		       created_at, updated_at, started_at, completed_at
		FROM jobs
		WHERE status IN ($1, $2)
		ORDER BY created_at DESC
	`

	rows, err := p.db.Query(query, JobStatusPending, JobStatusRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var idStr string

		err := rows.Scan(
			&idStr,
			&job.Type,
			&job.Status,
			&job.Progress,
			&job.CurrentStep,
			&job.TotalSteps,
			&job.Message,
			&job.Error,
			&job.Result,
			&job.CreatedAt,
			&job.UpdatedAt,
			&job.StartedAt,
			&job.CompletedAt,
		)

		if err != nil {
			return nil, err
		}

		job.ID, err = ulid.Parse(idStr)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// DeleteOldJobs deletes completed jobs older than the specified duration
func (p *PostgresDB) DeleteOldJobs(olderThan time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `
		DELETE FROM jobs
		WHERE status IN ($1, $2, $3)
		AND completed_at < $4
	`

	result, err := p.db.Exec(query, JobStatusCompleted, JobStatusFailed, JobStatusCancelled, cutoffTime)
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

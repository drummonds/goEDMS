package engine

import (
	"net/http"
	"strconv"

	"codeberg.org/hum3/godocs/database"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
)

// GetJob retrieves a job by ID
// @Summary Get job by ID
// @Description Retrieve details of a specific job by its ID
// @Tags Jobs
// @Accept json
// @Produce json
// @Param id path string true "Job ID (ULID)"
// @Success 200 {object} database.Job "Job details"
// @Failure 400 {object} map[string]interface{} "Invalid job ID"
// @Failure 404 {object} map[string]interface{} "Job not found"
// @Router /jobs/{id} [get]
func (serverHandler *ServerHandler) GetJob(c echo.Context) error {
	jobIDStr := c.Param("id")

	jobID, err := ulid.Parse(jobIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid job ID format",
		})
	}

	job, err := serverHandler.DB.GetJob(jobID)
	if err != nil {
		Logger.Error("Failed to get job", "jobID", jobIDStr, "error", err)
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Job not found",
		})
	}

	return c.JSON(http.StatusOK, job)
}

// GetRecentJobs retrieves recent jobs with pagination
// @Summary Get recent jobs
// @Description Retrieve a list of recent jobs with pagination
// @Tags Jobs
// @Accept json
// @Produce json
// @Param limit query int false "Number of jobs to return (default: 20)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {array} database.Job "List of jobs"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /jobs [get]
func (serverHandler *ServerHandler) GetRecentJobs(c echo.Context) error {
	limit := 20
	offset := 0

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := c.QueryParam("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	jobs, err := serverHandler.DB.GetRecentJobs(limit, offset)
	if err != nil {
		Logger.Error("Failed to get recent jobs", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve jobs",
		})
	}

	if jobs == nil {
		jobs = []database.Job{}
	}

	return c.JSON(http.StatusOK, jobs)
}

// GetActiveJobs retrieves all currently running or pending jobs
// @Summary Get active jobs
// @Description Retrieve all jobs that are currently running or pending
// @Tags Jobs
// @Accept json
// @Produce json
// @Success 200 {array} database.Job "List of active jobs"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /jobs/active [get]
func (serverHandler *ServerHandler) GetActiveJobs(c echo.Context) error {
	jobs, err := serverHandler.DB.GetActiveJobs()
	if err != nil {
		Logger.Error("Failed to get active jobs", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve active jobs",
		})
	}

	if jobs == nil {
		jobs = []database.Job{}
	}

	return c.JSON(http.StatusOK, jobs)
}

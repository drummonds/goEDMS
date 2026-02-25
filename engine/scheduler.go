package engine

import (
	"fmt"
	"log/slog"
	"time"

	database "github.com/drummonds/godocs/database"
	"github.com/robfig/cron/v3"
)

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// InitializeSchedules starts all the cron jobs (currently just one).
// Call InitJobContext before this.
func (serverHandler *ServerHandler) InitializeSchedules(db database.Repository) {
	serverConfig, err := database.FetchConfigFromDB(db)
	if err != nil {
		Logger.Error("Error reading db when initializing", "error", err)
	}

	// Recover any jobs stuck from a previous crash
	recovered, err := db.RecoverStuckJobs(30 * time.Minute)
	if err != nil {
		Logger.Error("Failed to recover stuck jobs", "error", err)
	} else if recovered > 0 {
		Logger.Info("Recovered stuck jobs from previous run", "count", recovered)
	}

	// Run ingress job immediately at startup (tracked)
	Logger.Info("Running ingress job at startup")
	go func() {
		job, err := db.CreateJob(database.JobTypeIngestion, "Startup ingestion")
		if err != nil {
			Logger.Error("Failed to create startup ingestion job", "error", err)
			return
		}
		ctx := serverHandler.RegisterJobCancel(job.ID)
		serverHandler.ingressJobFuncWithTracking(ctx, serverConfig, db, job.ID)
	}()

	c := cron.New()
	var ingressJob cron.Job
	ingressJob = cron.FuncJob(func() { serverHandler.ingressJobFunc(serverHandler.jobCtx, serverConfig, db) })
	ingressJob = cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(ingressJob)
	c.AddJob(fmt.Sprintf("@every %dm", serverConfig.IngressInterval), ingressJob)
	Logger.Info("Adding Ingress Job scheduler", "interval_minutes", serverConfig.IngressInterval)
	c.Start()

	serverHandler.cron = c
}

package engine

import (
	"fmt"
	"log/slog"

	"github.com/blevesearch/bleve"
	database "github.com/deranjer/goEDMS/database"
	"github.com/robfig/cron/v3"
)

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// InitializeSchedules starts all the cron jobs (currently just one)
func (serverHandler *ServerHandler) InitializeSchedules(db database.DBInterface, searchDB bleve.Index) {
	serverConfig, err := database.FetchConfigFromDB(db)
	if err != nil {
		fmt.Println("Error reading db when initializing")
	}

	// Run ingress job immediately at startup in a goroutine
	Logger.Info("Running ingress job at startup")
	go serverHandler.ingressJobFunc(serverConfig, db, searchDB)

	c := cron.New()
	var ingressJob cron.Job
	ingressJob = cron.FuncJob(func() { serverHandler.ingressJobFunc(serverConfig, db, searchDB) })
	ingressJob = cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(ingressJob) //ensure we don't kick off another if old one is still running
	c.AddJob(fmt.Sprintf("@every %dm", serverConfig.IngressInterval), ingressJob)
	//c.AddJob("@every 1m", ingressJob)
	Logger.Info("Adding Ingress Job scheduler", "interval_minutes", serverConfig.IngressInterval)
	c.Start()
}

package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"io/ioutil"

	"github.com/benbjohnson/clock"
	"github.com/cloudfoundry/uptimer/cfCmdGenerator"
	"github.com/cloudfoundry/uptimer/cfWorkflow"
	"github.com/cloudfoundry/uptimer/cmdRunner"
	"github.com/cloudfoundry/uptimer/config"
	"github.com/cloudfoundry/uptimer/measurement"
	"github.com/cloudfoundry/uptimer/orchestrator"
)

func main() {
	configPath := flag.String("configFile", "", "Path to the config file")
	flag.Parse()
	if *configPath == "" {
		log.Fatalln("Error: '-configFile' flag required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalln(err)
	}

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime|log.LUTC)
	stdOutAndErrRunner := cmdRunner.New(os.Stdout, os.Stderr, io.Copy)
	discardOutputRunner := cmdRunner.New(ioutil.Discard, ioutil.Discard, io.Copy)
	cfCmdGenerator := cfCmdGenerator.New()
	workflow := cfWorkflow.New(cfg.CF, cfCmdGenerator)
	measurements := []measurement.Measurement{
		measurement.NewAvailability(
			workflow.AppUrl(),
			time.Second,
			clock.New(),
			&http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			},
		),
		measurement.NewRecentLogs(
			10*time.Second,
			clock.New(),
			workflow.RecentLogs,
			discardOutputRunner,
		),
	}

	orc := orchestrator.New(cfg.While, logger, workflow, stdOutAndErrRunner, measurements)

	logger.Println("Setting up")
	if err := orc.Setup(); err != nil {
		logger.Println("Failed setup:", err)
		TearDownAndExit(orc, logger, 1)
	}

	exitCode, err := orc.Run()
	if err != nil {
		logger.Println("Failed run:", err)
		TearDownAndExit(orc, logger, 1)
	}

	TearDownAndExit(orc, logger, exitCode)
}

func TearDownAndExit(orc orchestrator.Orchestrator, logger *log.Logger, exitCode int) {
	logger.Println("Tearing down")
	if err := orc.TearDown(); err != nil {
		logger.Fatalln("Failed teardown:", err)
	}
	os.Exit(exitCode)
}

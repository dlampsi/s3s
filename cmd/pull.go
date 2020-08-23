package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"s3s/server"
	"s3s/syncer"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
)

var (
	pullCmd = &cobra.Command{
		Use:   "pull [s3_path] [local_path]",
		Short: "Syncronise data from S3 to localhost",
		Args:  pullCmdArgs,
		Run:   pullCmdRun,
	}
	flagRunOncePull    bool
	flagMetricPortPull int
	flagPullInterval   int64
)

func init() {
	rootCmd.AddCommand(pullCmd)
	pullCmd.PersistentFlags().BoolVar(&flagRunOncePull, "run-once", false, "Run cmd one time")
	pullCmd.PersistentFlags().IntVarP(&flagMetricPortPull, "metrics-port", "p", 8085, "Port for metrics exporter binds on")
	pullCmd.PersistentFlags().Int64VarP(&flagPullInterval, "interval", "i", 5, "Interval to pull data from S3 (in secconds)")
}

func pullCmdArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return errors.New("Command requires 2 arguments: s3 path and local direcotry path")
	}
	return nil
}

func pullCmdRun(cmd *cobra.Command, args []string) {
	// Logger
	logger, err := getLogger()
	if err != nil {
		fmt.Printf("Can't init logger: %s\n", err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()
	log := logger.Sugar().Named("cmd")

	svcOpts := []syncer.SyncerServiceOption{
		syncer.WithLogger(logger),
	}

	var r *prometheus.Registry
	if !flagRunOncePull {
		r = prometheus.NewRegistry()
		svcOpts = append(svcOpts, syncer.WithPrometheusRegistry(r))
	}

	svc, err := syncer.NewSyncerService(
		&syncer.SyncerConfig{
			RemoteURI:  args[0],
			LocalDir:   args[1],
			TempDir:    flagTempDir,
			S3Endpoint: flagS3Endpoint,
			DisableSSL: flagDisableSSL,
		},
		svcOpts...,
	)
	if err != nil {
		log.Fatal("Can't create syncer service: ", err.Error())
	}

	svc.InitHashCache()

	if flagRunOncePull {
		log.Info("Running once...")
		if err := svc.Pull(); err != nil {
			log.Fatal(err)
		}
		return
	}

	go func() {
		// Init run
		if err := svc.Pull(); err != nil {
			log.Fatal(err)
		}
		// Run by interval
		interval := time.Duration(flagPullInterval) * time.Second
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				start := time.Now()
				log.Info("Pull from S3 started")
				if err := svc.Pull(); err != nil {
					log.Error(err)
				}
				syncTime := time.Now().Sub(start)
				// syncTimeMetric.Set(float64(syncTime / time.Millisecond))
				log.Infof("Pull finished in %v sec.", syncTime)
			}
		}
	}()

	httpAddr := fmt.Sprintf(":%d", flagMetricPortPull)
	httpSrv := server.NewServer(httpAddr, server.WithLogger(logger), server.WithRegistry(r))
	httpSrv.Serve()
}

package cmd

import (
	"fmt"
	"os"
	"s3s/logging"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	rootCmd = &cobra.Command{
		Use:   "s3s",
		Short: "Synchronizes your data from S3",
	}
	flagVerbose    bool
	flagS3Endpoint string
	flagTempDir    string
	flagDisableSSL bool
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose output")
	pullCmd.PersistentFlags().StringVar(&flagTempDir, "temp-dir", "tmp", "Work directory to stopre temporary files")
	rootCmd.PersistentFlags().StringVarP(&flagS3Endpoint, "s3-endpoint", "e", "", "Custom S3 endpoint URL")
	rootCmd.PersistentFlags().BoolVar(&flagDisableSSL, "disable-ssl", false, "Disable SSL in S3 connection")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getLogger() (*zap.Logger, error) {
	cfg := &logging.Config{
		Level:        "info",
		EnableColors: true,
	}
	if flagVerbose {
		cfg.Level = "debug"
	}
	return logging.NewZapLogger(cfg)
}

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configPath string

func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "xray-master",
	Short: "Subscription master server for xray-node clusters",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default /etc/xray-master/config.yaml)")
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newNodeCmd())
	rootCmd.AddCommand(newUserCmd())
	rootCmd.AddCommand(newSyncCmd())
}

func loadConfigPath() string {
	if configPath != "" {
		return configPath
	}
	return ""
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

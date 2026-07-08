package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync state to VPN nodes",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "users",
		Short: "Provision all users on all registered nodes",
		Long: `Re-runs client provisioning for every enabled user on every enabled
inbound of every registered ready node. Subscription profiles only affect
what appears in /sub/{token}, not which nodes receive clients.`,
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			result, err := master.SyncAllUsers()
			if err != nil {
				fatal(err)
			}
			fmt.Printf("users synced: %d\n", result.UsersSynced)
			if len(result.NodeErrors) > 0 {
				fmt.Println("errors:")
				for k, v := range result.NodeErrors {
					fmt.Printf("  %s: %s\n", k, v)
				}
			}
		},
	})
	return cmd
}

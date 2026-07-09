package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thethoughtcriminal/xray-master/internal/service"
)

func newNodeTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Enroll tokens for node self-registration",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a one-time enroll token",
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			name, _ := cmd.Flags().GetString("name")
			ttl, _ := cmd.Flags().GetInt("ttl-hours")
			result, err := master.CreateEnrollToken(service.CreateEnrollTokenInput{
				Name: name, TTLHours: ttl,
			})
			if err != nil {
				fatal(err)
			}
			fmt.Printf("Enroll token for node %q (expires %s)\n\n", result.Name, result.ExpiresAt.Format("2006-01-02 15:04 UTC"))
			fmt.Printf("Token (shown once):\n  %s\n\n", result.Token)
			fmt.Println("Run on the VPS (after xray-node install):")
			fmt.Printf("  %s\n\n", result.JoinCmd)
			fmt.Println("Or via curl on the node:")
			fmt.Printf("  curl -fsSL https://raw.githubusercontent.com/themnts/xray-node/main/scripts/join.sh | sudo MASTER_URL=%s ENROLL_TOKEN=%s NODE_NAME=%s bash\n",
				result.MasterURL, result.Token, result.Name)
		},
	})
	create := cmd.Commands()[0]
	create.Flags().String("name", "", "node name (unique)")
	create.Flags().Int("ttl-hours", 0, "token lifetime in hours (default from config)")
	_ = create.MarkFlagRequired("name")
	return cmd
}

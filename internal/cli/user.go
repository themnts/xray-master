package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/service"
)

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage subscription users",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List users",
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			users, err := master.ListUsers()
			if err != nil {
				fatal(err)
			}
			cfg, _ := config.Load(loadConfigPath())
			for _, u := range users {
				subURL := subscriptionURL(cfg, u.SubToken)
				fmt.Printf("%s\t%s\t%s\tenabled=%v\n", u.ID, u.Email, subURL, u.Enabled)
			}
		},
	})
	add := &cobra.Command{
		Use:   "add",
		Short: "Add user on all nodes",
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			email, _ := cmd.Flags().GetString("email")
			userUUID, _ := cmd.Flags().GetString("uuid")
			result, err := master.AddUser(service.AddUserInput{Email: email, UUID: userUUID})
			if err != nil {
				fatal(err)
			}
			cfg, _ := config.Load(loadConfigPath())
			fmt.Printf("user added: %s\n", result.User.Email)
			fmt.Printf("subscription: %s\n", subscriptionURL(cfg, result.User.SubToken))
			if len(result.NodeErrors) > 0 {
				fmt.Println("node warnings:")
				for k, v := range result.NodeErrors {
					fmt.Printf("  %s: %s\n", k, v)
				}
			}
		},
	}
	add.Flags().String("email", "", "user email (same on all nodes)")
	add.Flags().String("uuid", "", "optional fixed UUID")
	_ = add.MarkFlagRequired("email")
	cmd.AddCommand(add)

	cmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Aggregate traffic stats",
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			email, _ := cmd.Flags().GetString("email")
			stats, err := master.UserStats(email)
			if err != nil {
				fatal(err)
			}
			fmt.Printf("%s up=%d down=%d\n", stats.Email, stats.Up, stats.Down)
			for name, t := range stats.ByNode {
				fmt.Printf("  %s (%s): up=%d down=%d\n", name, t.Inbound, t.Up, t.Down)
			}
		},
	})
	stats := cmd.Commands()[2]
	stats.Flags().String("email", "", "user email")
	_ = stats.MarkFlagRequired("email")

	return cmd
}

func subscriptionURL(cfg *config.Config, token string) string {
	if cfg == nil {
		return token
	}
	return strings.TrimRight(cfg.Server.PublicURL, "/") + "/sub/" + token
}

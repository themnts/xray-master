package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/db"
	"github.com/thethoughtcriminal/xray-master/internal/service"
)

func newNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage VPN nodes",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered nodes",
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			nodes, err := master.ListNodes()
			if err != nil {
				fatal(err)
			}
			for _, n := range nodes {
				fmt.Printf("%s\t%s\t%s\tenabled=%v\n", n.ID, n.Name, n.PublicHost, n.Enabled)
			}
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Register a node",
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			name, _ := cmd.Flags().GetString("name")
			apiURL, _ := cmd.Flags().GetString("api-url")
			apiKey, _ := cmd.Flags().GetString("api-key")
			host, _ := cmd.Flags().GetString("public-host")
			node, err := master.AddNode(service.AddNodeInput{
				Name: name, APIURL: apiURL, APIKey: apiKey, PublicHost: host,
			})
			if err != nil {
				fatal(err)
			}
			fmt.Printf("node added: %s (%s)\n", node.Name, node.ID)
		},
	})
	add := cmd.Commands()[1]
	add.Flags().String("name", "", "node name")
	add.Flags().String("api-url", "", "xray-node API base URL")
	add.Flags().String("api-key", "", "xray-node API key")
	add.Flags().String("public-host", "", "public hostname for client links")
	_ = add.MarkFlagRequired("name")
	_ = add.MarkFlagRequired("api-url")
	_ = add.MarkFlagRequired("api-key")
	_ = add.MarkFlagRequired("public-host")

	cmd.AddCommand(&cobra.Command{
		Use:   "remove [id]",
		Short: "Remove a node",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			if err := master.DeleteNode(args[0]); err != nil {
				fatal(err)
			}
			fmt.Println("node removed")
		},
	})
	return cmd
}

func openMaster() (*service.Master, func()) {
	cfg, err := config.Load(loadConfigPath())
	if err != nil {
		fatal(err)
	}
	conn, err := db.Open(cfg.Server.DBPath)
	if err != nil {
		fatal(err)
	}
	return service.New(cfg, conn), func() { _ = conn.Close() }
}

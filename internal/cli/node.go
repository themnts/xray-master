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
				fmt.Printf("%s\t%s\tip=%s\t%s\tstatus=%s\tenabled=%v\n",
					n.ID, n.Name, n.IP, n.PublicHost, n.Status, n.Enabled)
			}
		},
	})
	add := &cobra.Command{
		Use:   "add",
		Short: "Register a node (auto-provision via SSH when --ip is set)",
		Run: func(cmd *cobra.Command, args []string) {
			master, cleanup := openMaster()
			defer cleanup()
			name, _ := cmd.Flags().GetString("name")
			ip, _ := cmd.Flags().GetString("ip")
			apiURL, _ := cmd.Flags().GetString("api-url")
			apiKey, _ := cmd.Flags().GetString("api-key")
			host, _ := cmd.Flags().GetString("public-host")
			fmt.Printf("Adding node %q", name)
			if ip != "" {
				fmt.Printf(" (provisioning %s via SSH)...", ip)
			}
			fmt.Println()
			node, err := master.AddNode(service.AddNodeInput{
				Name: name, IP: ip, APIURL: apiURL, APIKey: apiKey, PublicHost: host,
			})
			if err != nil {
				fatal(err)
			}
			fmt.Printf("node added: %s (%s) status=%s api=%s\n", node.Name, node.ID, node.Status, node.APIURL)
			fmt.Println("If the node is new in subscription.profiles, run: xray-master sync users")
		},
	}
	add.Flags().String("name", "", "node name (must match subscription.profiles entries)")
	add.Flags().String("ip", "", "node VPS IP — master SSHs in and installs xray-node")
	add.Flags().String("api-url", "", "manual mode: xray-node API base URL")
	add.Flags().String("api-key", "", "manual mode: xray-node API key")
	add.Flags().String("public-host", "", "hostname in client links (default: node IP)")
	_ = add.MarkFlagRequired("name")
	cmd.AddCommand(add)

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

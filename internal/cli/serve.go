package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/thethoughtcriminal/xray-master/internal/api"
	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/db"
	"github.com/thethoughtcriminal/xray-master/internal/service"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start master HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(loadConfigPath())
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			conn, err := db.Open(cfg.Server.DBPath)
			if err != nil {
				return err
			}
			defer conn.Close()

			master := service.New(cfg, conn)
			server := api.New(cfg, master)
			fmt.Printf("xray-master listening on http://%s\n", cfg.Server.Listen)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() { errCh <- server.ListenAndServe() }()

			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = server.Shutdown(shutdownCtx)
				return nil
			case err := <-errCh:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				return nil
			}
		},
	}
}

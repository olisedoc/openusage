package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/janekbaraniewski/openusage/internal/web"
	"github.com/spf13/cobra"
)

func newWebCommand() *cobra.Command {
	var listen string
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Serve a local web dashboard of current usage snapshots",
		Long:  "Collects the same snapshots as `openusage export` and serves a three-column remaining-quota page. Bind loopback (default) and publish with Tailscale Serve, or bind a LAN address for same-Wi-Fi access without Serve.",
		Run: func(_ *cobra.Command, _ []string) {
			if listen == "" {
				listen = "127.0.0.1:8060"
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			srv := web.NewServer(listen)
			log.Printf("openusage web listening on http://%s/", listen)
			if err := srv.ListenAndServe(ctx); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:8060", "TCP address (use 127.0.0.1:8060 behind Tailscale Serve, or 192.168.x.x:8060 on LAN)")
	return cmd
}

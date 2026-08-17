// Command worker is a placeholder long-running background process.
// It logs a heartbeat every second so the deployment has a non-API
// service to deploy.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stdout, "worker: shutdown")
			return
		case <-t.C:
			fmt.Fprintln(os.Stdout, "worker: heartbeat", time.Now().UTC().Format(time.RFC3339))
		}
	}
}

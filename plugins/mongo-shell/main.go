// Command plugin is the GoNavi MongoShell backend. It speaks protocol v1 on
// stdio and runs user JavaScript with goja against a saved MongoDB connection.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	plugin "github.com/Syngnat/GoNavi/sdk/go"

	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/mongo-shell/internal/shell"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtime := shell.New()
	runtime.Logger = func(ctx context.Context, level, message string) {
		if host, ok := plugin.HostFromContext(ctx); ok {
			_ = host.Log(ctx, level, message)
		}
	}
	server := plugin.NewServer(plugin.Options{
		Handlers: map[string]plugin.Handler{
			"connect":    runtime.Connect,
			"disconnect": runtime.Disconnect,
			"ping":       runtime.Ping,
			"run":        runtime.RunCommand,
		},
		OnClose: runtime.Abort,
	})
	if err := server.ServeStdio(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

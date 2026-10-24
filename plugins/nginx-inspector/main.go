// Command plugin is the GoNavi Nginx Inspector backend. It speaks protocol v1
// on stdio and exposes read-only diagnostics for remote nginx instances over SSH.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	plugin "github.com/Syngnat/GoNavi/sdk/go"
	"github.com/Syngnat/GoNavi-Plugin-Market/plugins/nginx-inspector/internal/commands"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtime := commands.NewRuntime()
	server := plugin.NewServer(plugin.Options{
		Handlers: runtime.Handlers(),
		Initialize: func(ctx context.Context, params plugin.InitializeParams) error {
			runtime.SetLocale(params.Locale)
			return nil
		},
		OnClose: runtime.Close,
	})
	if err := server.ServeStdio(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

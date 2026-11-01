package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/desanv01/cuemender/internal/app"
)

func main() {
	os.Exit(run())
}

func run() int {
	flags := flag.NewFlagSet("cuemender", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dataDir := flags.String("data-dir", "", "data directory (default: user config directory / LiveCheck)")
	listen := flags.String("listen", "", "loopback listen address override, e.g. 127.0.0.1:8787")
	noBrowser := flags.Bool("no-browser", false, "do not open a browser; print the one-time control link")
	sessionOnly := flags.Bool("session-only-secrets", false, "keep secrets in memory for this process only")
	showVersion := flags.Bool("version", false, "print version information and exit")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "cuemender: unexpected arguments:", flags.Args())
		return 2
	}
	if *showVersion {
		fmt.Println(app.VersionString())
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, app.Options{
		DataDir:            *dataDir,
		ListenAddress:      *listen,
		NoBrowser:          *noBrowser,
		SessionOnlySecrets: *sessionOnly,
		Stdout:             os.Stdout,
		Stderr:             os.Stderr,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "cuemender:", err)
		return 1
	}
	if err := a.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "cuemender:", err)
		return 1
	}
	return 0
}

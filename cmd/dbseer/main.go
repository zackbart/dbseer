// Command dbseer is a lightweight, browser-based Postgres GUI for dev environments.
//
// Usage:
//
//	dbseer                  # auto-discover from CWD and open browser
//	dbseer --url <url>      # override discovery
//	dbseer --which          # print discovered connection and exit
//	dbseer --readonly       # disable edit UI and force read-only transactions
//	dbseer history          # view audit log
//	dbseer --version        # print version
//
// See the README for full flag reference and safety-rail behavior.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/zackbart/dbseer/internal/db"
	"github.com/zackbart/dbseer/internal/discover"
	"github.com/zackbart/dbseer/internal/safety"
	"github.com/zackbart/dbseer/internal/server"
)

func run() error {
	// 1. Parse flags.
	fs := flag.NewFlagSet("dbseer", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dbseer [flags] [subcommand]\n\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  history    View the audit log\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	var (
		urlFlag      string
		hostFlag     string
		portFlag     int
		allowRemote  bool
		allowProd    bool
		readonlyFlag bool
		debugFlag    bool
		quietFlag    bool
		whichFlag    bool
		dryRunFlag   bool
		devFlag      bool
		noOpenFlag   bool
		envFlag      string
		versionFlag  bool
	)

	fs.StringVar(&urlFlag, "url", "", "Override discovery with a literal Postgres URL")
	fs.StringVar(&hostFlag, "host", "127.0.0.1", "HTTP bind address")
	fs.IntVar(&portFlag, "port", 4983, "HTTP bind port")
	fs.BoolVar(&allowRemote, "allow-remote", false, "Allow non-localhost DB hosts")
	fs.BoolVar(&allowProd, "allow-prod", false, "Allow prod-pattern DB hosts")
	fs.BoolVar(&readonlyFlag, "readonly", false, "Disable edit UI and set session default_transaction_read_only=on")
	fs.BoolVar(&debugFlag, "debug", false, "Set log level to Debug (default: Info)")
	fs.BoolVar(&quietFlag, "quiet", false, "Set log level to Warn")
	fs.BoolVar(&whichFlag, "which", false, "Run discovery, print resolved connection, and exit")
	fs.BoolVar(&dryRunFlag, "dry-run", false, "Alias for --which")
	fs.BoolVar(&devFlag, "dev", false, "Force dev mode (used internally by air under -tags dev builds)")
	fs.BoolVar(&noOpenFlag, "no-open", false, "Don't open the browser automatically")
	fs.StringVar(&envFlag, "env", "", "Select a .dbseer.json environment by name")
	fs.BoolVar(&versionFlag, "version", false, "Print version and exit")

	// -v as alias for --version.
	fs.BoolVar(&versionFlag, "v", false, "Print version and exit (alias for --version)")

	args := os.Args[1:]

	// Handle subcommands before flag parsing so flags don't steal positional args.
	if len(args) > 0 && args[0] == "history" {
		return runHistory(args[1:])
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// --version
	if versionFlag {
		fmt.Printf("dbseer v%s\n", version)
		return nil
	}

	// --which is also aliased to --dry-run.
	if dryRunFlag {
		whichFlag = true
	}

	// 2. Configure slog based on --debug/--quiet.
	var logLevel slog.Level
	switch {
	case debugFlag:
		logLevel = slog.LevelDebug
	case quietFlag:
		logLevel = slog.LevelWarn
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	_ = devFlag // used at build time via -tags dev; noted here for completeness

	// 4. Discover.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// If --url is set, override discovery.
	var source discover.Source
	if urlFlag != "" {
		source = discover.Source{
			Kind: discover.SourceFlag,
			URL:  urlFlag,
		}
	} else {
		source, err = resolveSource(cwd, envFlag, os.Stdin, os.Stdout)
		if err != nil {
			return fmt.Errorf("discovery failed: %w", err)
		}
	}

	// If SourceNone and no --url, fail with actionable error.
	if source.Kind == discover.SourceNone {
		return fmt.Errorf(
			"no Postgres connection found — set DATABASE_URL in a .env file, " +
				"use --url to specify one explicitly, or add a .dbseer.json config",
		)
	}

	// 5. If --which, render and exit 0.
	if whichFlag {
		source.Render(os.Stdout)
		return nil
	}

	// 6. Parse the URL.
	info, err := safety.Parse(source.URL)
	if err != nil {
		return fmt.Errorf("parsing DSN: %w", err)
	}

	// 7. ValidateURL.
	if err := safety.ValidateURL(info, safety.Options{
		AllowRemote: allowRemote,
		AllowProd:   allowProd,
		Readonly:    readonlyFlag,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// 8. ValidateBind.
	if err := safety.ValidateBind(hostFlag); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// 9. NewPool.
	pool, err := db.NewPool(ctx, source.URL, readonlyFlag)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// 10. NewSchemaCache.
	cache := db.NewSchemaCache(30 * time.Second)

	// 11. NewLogger.
	logDir := safety.ResolveLogDir(source.ProjectRoot)
	auditLog, err := safety.NewLogger(logDir)
	if err != nil {
		// Non-fatal: log and continue without audit logging.
		logger.Warn("could not create audit log", "dir", logDir, "err", err)
	}

	// 12. server.New.
	srv := server.New(server.Config{
		Pool:     pool,
		Cache:    cache,
		Source:   source,
		AuditLog: auditLog,
		Readonly: readonlyFlag,
		Version:  version,
		Logger:   logger,
	})

	// 13. Start http.Server on host:port with timeouts to prevent slowloris.
	addr := net.JoinHostPort(hostFlag, fmt.Sprintf("%d", portFlag))
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("dbseer running", "url", fmt.Sprintf("http://%s", addr), "readonly", readonlyFlag)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// 14. If !noOpen, launch browser.
	if !noOpenFlag {
		browserURL := fmt.Sprintf("http://%s", addr)
		if err := openBrowser(browserURL); err != nil {
			logger.Debug("could not open browser", "err", err)
		}
	}

	// 15. Graceful shutdown on SIGINT/SIGTERM: 5 second timeout, close pool.
	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Warn("shutdown error", "err", err)
		}
		pool.Close()
		return nil
	}
}

// openBrowser opens the given URL in the default browser.
// Non-fatal if it fails.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		// Linux and other Unix-like systems.
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

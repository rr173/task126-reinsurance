// Command task126-reinsurance runs the reinsurance ceding & loss allocation
// engine either as an HTTP service or in --smoke-test mode (which exercises the
// full business loop in-process against an in-memory SQLite and exits).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task126-reinsurance/internal/httpapi"
	"task126-reinsurance/internal/selfcheck"
	"task126-reinsurance/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "task126-reinsurance: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("task126-reinsurance", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	dsn := fs.String("dsn", "reinsurance.db", "SQLite DSN (use ':memory:' for ephemeral)")
	smoke := fs.Bool("smoke-test", false, "run in-process selfcheck and exit (uses an isolated in-memory DB)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()

	// The smoke test always uses an isolated in-memory database so the
	// business loop runs deterministically without colliding with persisted
	// state from a default DSN.
	dbDSN := *dsn
	if *smoke {
		dbDSN = ":memory:"
	}
	s, err := store.Open(ctx, dbDSN)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	if *smoke {
		if err := selfcheck.Run(ctx, s); err != nil {
			return fmt.Errorf("selfcheck failed: %w", err)
		}
		fmt.Println("smoke-test: OK")
		return nil
	}

	api, err := httpapi.New(s)
	if err != nil {
		return fmt.Errorf("build api: %w", err)
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("task126-reinsurance listening on %s (dsn=%s)", *addr, *dsn)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen error: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	shutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}

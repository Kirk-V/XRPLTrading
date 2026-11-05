// Command trader serves the read-only market dashboard on localhost.
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
	"time"

	"github.com/Kirk-V/XRPLTrading/internal/config"
	"github.com/Kirk-V/XRPLTrading/internal/server"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	network := flag.String("network", string(config.Testnet), "XRPL network: testnet or mainnet (read-only)")
	port := flag.Int("port", 8080, "localhost HTTP port")
	webDir := flag.String("web", "web/dist", "built frontend directory")
	flag.Parse()
	if *port < 1 || *port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	cfg, err := config.ForNetwork(config.Network(*network))
	if err != nil {
		return err
	}
	client, err := xrpl.NewAPIClient(cfg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	service := server.New(client, cfg)
	go service.Run(ctx)
	httpServer := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", *port), Handler: service.Handler(*webDir), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	log.Printf("XRPL Trading: http://localhost:%d — %s, read-only", *port, cfg.Network)
	err = httpServer.ListenAndServe()
	stop()
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

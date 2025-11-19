package main

// tessad: Tessa daemon CLI
//
// This is a minimal, self-contained daemon that:
// - Starts an HTTP API server on :7979
// - Exposes the same API over a Unix domain socket at /var/run/tessa.sock
// - Gracefully shuts down on SIGINT/SIGTERM and removes the Unix socket file
//

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Fyve-Labs/tessa-daemon/internal/pubsub"
	"github.com/Fyve-Labs/tessa-daemon/internal/server"
	"github.com/Fyve-Labs/tessa-daemon/internal/subscribers"
	"github.com/Fyve-Labs/tessa-daemon/internal/tunnel"
	chclient "github.com/jpillora/chisel/client"
)

const (
	defaultHTTPAddr = ":7979"
	defaultUnixSock = "/var/run/tessa.sock"
	defaultDataDir  = "/var/lib/tessa"
)

func main() {
	var (
		httpAddr = flag.String("http-addr", envOr("TESSAD_HTTP_ADDR", defaultHTTPAddr), "HTTP listen address")
		unixSock = flag.String("unix-sock", envOr("TESSAD_UNIX_SOCK", defaultUnixSock), "Unix domain socket path")
		dataDir  = flag.String("data-dir", envOr("TESSAD_DATA_DIR", defaultDataDir), "Base data directory (credentials, etc.)")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "tessad ", log.LstdFlags|log.Lmicroseconds)

	// Determine credentials directory only if writable; else leave empty for stateless mode
	credentialsDir := ""

	// Ensure data dir exists and warn if not writable (stateless mode)
	dataWritable := false
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		if isPermErr(err) {
			logger.Printf("warning: data dir %s is not writable: %v", *dataDir, err)
		} else {
			logger.Printf("warning: failed to prepare data dir %s: %v (continuing)", *dataDir, err)
		}
	} else if canWriteDir(*dataDir) {
		dataWritable = true
	} else {
		logger.Printf("warning: data dir %s is not writable (running stateless)", *dataDir)
	}

	if dataWritable {
		cd := filepath.Join(*dataDir, "credentials")
		// Try to create the credentials dir and ensure writability
		if err := os.MkdirAll(cd, 0o700); err != nil {
			logger.Printf("warning: cannot create credentials dir %s: %v (stateless mode without persistence)", cd, err)
		} else if canWriteDir(cd) {
			credentialsDir = cd
		} else {
			logger.Printf("warning: credentials dir %s is not writable (stateless mode without persistence)", cd)
		}
	}

	ps := pubsub.NewInMemory()

	// Tunnel manager used by subscribers to manage SSH tunnels
	// Start the tunnel manager in background once it's ready; cancel on shutdown
	mgrCtx, mgrCancel := context.WithCancel(context.Background())
	mgr := tunnel.NewManager(mgrCtx, logger)

	// Attempt to load TLS from existing credentials on startup
	if credentialsDir != "" {
		certPath := filepath.Join(credentialsDir, "secure_tunnel.crt")
		keyPath := filepath.Join(credentialsDir, "secure_tunnel.key")
		caPath := filepath.Join(credentialsDir, "root_ca.crt")
		if fileExists(certPath) && fileExists(keyPath) && fileExists(caPath) {
			// Set TLS using file paths; if Start is called later after server/remotes are set,
			// manager will be Ready and use this TLS configuration.
			mgr.SetTLS(chclient.TLSConfig{Cert: certPath, Key: keyPath, CA: caPath})
			logger.Printf("loaded tunnel TLS from credentials from disk: %s", credentialsDir)
		}
	}

	defer mgrCancel()
	go func() {
		if mgr.Ready() {
			logger.Printf("tunnel manager: configuration ready, starting client...")
			if err := mgr.Start(); err != nil {
				logger.Printf("tunnel manager: start error: %v", err)
			}
		}
	}()

	stopSubs := subscribers.Start(ps, logger, credentialsDir, mgr)
	defer stopSubs()
	defer ps.Close()

	api := server.New(ps)

	// TCP server
	httpSrv := &http.Server{Addr: *httpAddr, Handler: api}

	// Optionally start Unix socket server (stateless if not permitted)
	var (
		unixLn   net.Listener
		unixUsed bool
	)

	if *unixSock != "" {
		ln, err := server.PrepareUnixSocket(*unixSock)
		if err != nil {
			if isPermErr(err) {
				logger.Printf("warning: unix socket disabled due to permission error at %s: %v", *unixSock, err)
			} else {
				logger.Printf("warning: failed to prepare unix socket at %s: %v (continuing without it)", *unixSock, err)
			}
		} else {
			unixLn = ln
			unixUsed = true
		}
	}

	// Start servers
	tcpErrCh := make(chan error, 1)
	unixErrCh := make(chan error, 1)
	go func() {
		logger.Printf("HTTP listening on %s", *httpAddr)
		tcpErrCh <- httpSrv.ListenAndServe()
	}()
	if unixUsed && unixLn != nil {
		go func() {
			logger.Printf("Unix socket listening on %s", *unixSock)
			unixErrCh <- http.Serve(unixLn, api)
		}()
	}

	// Signal handling
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Printf("received signal: %v, shutting down...", sig)
		// Stop tunnel manager first for graceful shutdown of client
		mgrCancel()
	case err := <-tcpErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("HTTP server error: %v", err)
		}
	case err := <-unixErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("Unix socket server error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	if unixUsed {
		_ = unixLn.Close()
		_ = os.Remove(*unixSock)
	}
	logger.Printf("shutdown complete")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// fileExists reports whether the named file exists.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

// isPermErr reports if err is a permission-related error.
func isPermErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EROFS)
}

// canWriteDir attempts to create and remove a temp file in dir.
func canWriteDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".permcheck-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

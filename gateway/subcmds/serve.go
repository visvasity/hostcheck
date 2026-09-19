// Copyright (c) 2026 Visvasity LLC

package subcmds

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"

	"github.com/visvasity/appdirs"
	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/internal/buildinfo"
	"github.com/visvasity/httphelp"
	"github.com/visvasity/logdir"
	"github.com/visvasity/runcmd"
)

// ServeCmd runs the Gateway web-frontend server. Wrapped by runcmd.Wrap it gains
// -background, -restart, and -self-monitor for systemd/daemon use.
//
// This is scaffolding: it stands up the HTTP(S) server, logging, health check,
// and daemon lifecycle. The subscriber UI pages (GATEWAY.md) and report
// ingestion are added in later milestones; for now "/" serves a placeholder.
type ServeCmd struct {
	dirs appdirs.Config

	httpPort  int
	tlsPort   int
	pprofPort int

	tlsCertPath string
	tlsKeyPath  string
	tlsConfig   *tls.Config

	logDebug  bool
	logStderr bool
}

func (c *ServeCmd) Purpose() string {
	return "Run the Gateway web-frontend server"
}

func (c *ServeCmd) Command() (string, *flag.FlagSet, cli.CmdFunc) {
	c.dirs.Program = "gateway"
	fset := new(flag.FlagSet)
	c.dirs.SetFlags(fset, &c.dirs)
	fset.IntVar(&c.httpPort, "http-port", 8080, "TCP port for the HTTP server")
	fset.IntVar(&c.tlsPort, "tls-port", 0, "TCP port for the HTTPS server (0 disables)")
	fset.IntVar(&c.pprofPort, "pprof-port", 0, "localhost-only port for net/http/pprof (0 disables)")
	fset.StringVar(&c.tlsCertPath, "tls-cert", "", "path to the TLS certificate file")
	fset.StringVar(&c.tlsKeyPath, "tls-key", "", "path to the TLS certificate key file")
	fset.BoolVar(&c.logDebug, "log-debug", false, "When true, enables debug logging")
	fset.BoolVar(&c.logStderr, "logtostderr", false, "When true, logs are written only to stderr")
	return "run", fset, c.run
}

// LocksDir tells runcmd where to place its daemon lock/socket files.
func (c *ServeCmd) LocksDir() string { return c.dirs.RuntimeDir }

func (c *ServeCmd) Check(ctx context.Context) error {
	c.dirs.Program = "gateway"
	if err := c.dirs.Check(ctx); err != nil {
		return err
	}
	if c.httpPort <= 0 {
		return fmt.Errorf("http port (-http-port) must be positive")
	}
	if (c.tlsCertPath == "") != (c.tlsKeyPath == "") {
		return fmt.Errorf("both -tls-cert and -tls-key are required to enable HTTPS")
	}
	if c.tlsCertPath != "" {
		cert, err := tls.LoadX509KeyPair(c.tlsCertPath, c.tlsKeyPath)
		if err != nil {
			return fmt.Errorf("could not load TLS keypair: %w", err)
		}
		c.tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}
	if c.tlsConfig != nil && c.tlsPort <= 0 {
		return fmt.Errorf("TLS port (-tls-port) must be positive when a certificate is given")
	}
	if c.pprofPort < 0 {
		return fmt.Errorf("pprof port (-pprof-port) cannot be negative")
	}
	if r, ok := runcmd.FromContext(ctx); ok && r.Background && c.logStderr {
		return fmt.Errorf("logging to stderr (-logtostderr) cannot be used with -background")
	}
	return nil
}

func (c *ServeCmd) run(ctx context.Context, args []string) error {
	if err := c.Check(ctx); err != nil {
		return err
	}

	level := slog.LevelInfo
	if c.logDebug {
		level = slog.LevelDebug
	}
	slog.SetLogLoggerLevel(level)
	if !c.logStderr {
		sink, err := logdir.Open(logdir.Config{Dir: c.dirs.LogDir, Level: level})
		if err != nil {
			return err
		}
		slog.SetDefault(sink.Logger(""))
	}

	var opts []httphelp.Option
	if c.tlsConfig != nil {
		opts = append(opts, httphelp.WithTLSConfig(c.tlsConfig))
	}
	server, err := httphelp.NewServer(opts...)
	if err != nil {
		return err
	}
	defer server.Close()

	// Application routes. UI pages (GATEWAY.md, built on pagemaker) mount here in
	// a later milestone; for now the root is a placeholder.
	server.Handle("/healthz", http.HandlerFunc(healthHandler))
	server.Handle("/", http.HandlerFunc(rootHandler))

	httpAddr, err := server.StartTCP(net.JoinHostPort("", strconv.Itoa(c.httpPort)), false)
	if err != nil {
		return fmt.Errorf("could not start HTTP listener: %w", err)
	}
	defer server.Stop(httpAddr)
	slog.Info("gateway HTTP listening", "addr", httpAddr.String())

	if c.tlsConfig != nil {
		tlsAddr, err := server.StartTCP(net.JoinHostPort("", strconv.Itoa(c.tlsPort)), true)
		if err != nil {
			return fmt.Errorf("could not start HTTPS listener: %w", err)
		}
		defer server.Stop(tlsAddr)
		slog.Info("gateway HTTPS listening", "addr", tlsAddr.String())
	}

	// pprof is exposed only on localhost, when enabled.
	if c.pprofPort > 0 {
		host := net.JoinHostPort("localhost", strconv.Itoa(c.pprofPort))
		addr, err := server.StartTCP(host, false)
		if err != nil {
			return fmt.Errorf("could not start pprof listener: %w", err)
		}
		defer server.Stop(addr)
		server.Handle("localhost/debug/pprof/", http.HandlerFunc(pprof.Index))
		server.Handle("127.0.0.1/debug/pprof/", http.HandlerFunc(pprof.Index))
		slog.Info("pprof listening on localhost", "addr", addr.String())
	}

	// Report successful initialization to the foreground/monitor process so
	// -background can succeed or fail fast.
	runcmd.Report(ctx, nil)
	slog.Info("gateway started", "version", buildinfo.Version())

	<-ctx.Done()
	slog.Info("gateway stopping", "cause", context.Cause(ctx))
	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "ok")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>Visvasity Gateway</title>
<body style="font:15px system-ui;margin:6rem auto;max-width:36rem;padding:0 1rem">
<h1>Visvasity Gateway</h1>
<p>The web frontend is not implemented yet. This is the server scaffolding.</p>
<p style="color:#666">%s</p>
</body>`, buildinfo.Version())
}

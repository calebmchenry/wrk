// Package web serves one fixed project over loopback HTTP. Reads never take a
// writer lock; requests cannot change the selected filesystem root.
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"wrk/internal/diagnostic"
	"wrk/internal/project"
)

const (
	DefaultPort        = 7331
	ReadTimeout        = 15 * time.Second
	ShutdownTimeout    = 3 * time.Second
	MaxBodyBytes       = 1 << 20
	MaxConcurrentReads = 4
)

func readLimits() project.ReadLimits {
	return project.ReadLimits{FileBytes: 2 << 20, TotalBytes: 32 << 20, DirectoryEntries: 20000}
}

type Server struct {
	listener net.Listener
	http     *http.Server
	URL      string
	Root     string
}

// Listen validates before binding; port zero asks the OS for an available port.
// The caller must Run or Close the returned server.
func Listen(ctx context.Context, root string, port int) (*Server, []diagnostic.Diagnostic) {
	if port < 0 || port > 65535 {
		return nil, []diagnostic.Diagnostic{diagnostic.New("USAGE", "port must be from 0 to 65535", "")}
	}
	if !filepath.IsAbs(root) {
		return nil, []diagnostic.Diagnostic{diagnostic.New("INVALID_PROJECT", "server requires an absolute selected project root", "")}
	}
	readCtx, cancel := context.WithTimeout(ctx, ReadTimeout)
	snapshot := project.LoadContext(readCtx, root, readLimits())
	cancel()
	if len(snapshot.Diagnostics) > 0 {
		return nil, snapshot.Diagnostics
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, []diagnostic.Diagnostic{diagnostic.New("LISTEN", fmt.Sprintf("cannot listen on 127.0.0.1:%d: %v; stop the other listener or choose --port (0 allocates a port)", port, err), "")}
	}
	authority := listener.Addr().String()
	s := &Server{listener: listener, Root: root, URL: "http://" + authority + "/"}
	s.http = &http.Server{
		Handler:           newHandler(root, authority),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       ReadTimeout,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
		// Request failures are returned as API diagnostics, not unstructured CLI logs.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	return s, nil
}

func (s *Server) Close() error {
	// Close the listener explicitly even when Run has not started yet.
	_ = s.listener.Close()
	return s.http.Close()
}

// Run returns after cancellation has closed the listener and active connections.
// BaseContext cancels handlers (including future streams) before bounded draining.
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer s.Close()
	s.http.BaseContext = func(net.Listener) context.Context { return ctx }
	ended := make(chan error, 1)
	go func() { ended <- s.http.Serve(s.listener) }()
	select {
	case err := <-ended:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		cancel()
		drain, stop := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer stop()
		err := s.http.Shutdown(drain)
		if err != nil {
			_ = s.http.Close()
		}
		serveErr := <-ended
		if err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
}

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"wrk/internal/diagnostic"
	"wrk/internal/web"
)

func runServe(r Request, root string, out, errout io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, r, root, out, errout, openBrowser)
}

func serve(ctx context.Context, r Request, root string, out, errout io.Writer, open func(context.Context, string) error) int {
	e := Envelope{SchemaVersion: 1, Command: "serve", ProjectRoot: &root, Errors: []diagnostic.Diagnostic{}}
	s, ds := web.Listen(ctx, root, r.Port)
	if len(ds) > 0 {
		e.Errors = ds
		return render(out, errout, r, e, "", 1)
	}
	defer s.Close()
	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	ended := make(chan error, 1)
	go func() { ended <- s.Run(ctx) }()
	e.OK = true
	e.Result = map[string]any{"event": "started", "url": s.URL}
	if render(out, errout, r, e, "Serving "+root+"\n"+s.URL+"\nPress Ctrl-C to stop.", 0) != 0 {
		return 1
	}
	if r.Open {
		openCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := open(openCtx, s.URL)
		cancel()
		if err != nil && ctx.Err() == nil {
			warning := diagnostic.New("OPEN_BROWSER", "could not open browser: "+err.Error()+"; open "+s.URL+" manually", "")
			e.Result = map[string]any{"event": "warning", "url": s.URL, "warning": warning}
			if r.JSON {
				if render(out, errout, r, e, "", 0) != 0 {
					return 1
				}
			} else if _, err := fmt.Fprintln(errout, Safe(warning.Code+": "+warning.Message)); err != nil {
				return 1
			}
		}
	}
	if err := <-ended; err != nil {
		e.OK = false
		e.Errors = []diagnostic.Diagnostic{diagnostic.New("SERVE", err.Error(), "")}
		e.Result = map[string]any{"event": "error", "url": s.URL}
		return render(out, errout, r, e, "", 1)
	}
	e.Result = map[string]any{"event": "stopped", "url": s.URL}
	return render(out, errout, r, e, "Stopped "+s.URL, 0)
}

func openBrowser(ctx context.Context, url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	// No shell, user command templates, or browser flags from the request.
	return exec.CommandContext(ctx, name, url).Run()
}

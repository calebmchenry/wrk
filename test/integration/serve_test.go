package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type serveProcess struct {
	cmd    *exec.Cmd
	events chan envelope
	done   chan error
	stderr bytes.Buffer
	url    string
}

func startServe(t *testing.T, executable, cwd string, args ...string) *serveProcess {
	t.Helper()
	return startServeEnv(t, executable, cwd, nil, args...)
}

func startServeEnv(t *testing.T, executable, cwd string, env []string, args ...string) *serveProcess {
	t.Helper()
	p := &serveProcess{events: make(chan envelope, 8), done: make(chan error, 1)}
	p.cmd = exec.Command(executable, args...)
	p.cmd.Dir = cwd
	p.cmd.Env = env
	p.cmd.Stderr = &p.stderr
	reader, writer := io.Pipe()
	p.cmd.Stdout = writer
	go func() {
		defer close(p.events)
		defer reader.Close()
		dec := json.NewDecoder(reader)
		for {
			var e envelope
			if err := dec.Decode(&e); err != nil {
				return
			}
			p.events <- e
		}
	}()
	if err := p.cmd.Start(); err != nil {
		writer.Close()
		t.Fatal(err)
	}
	go func() { err := p.cmd.Wait(); writer.Close(); p.done <- err }()
	t.Cleanup(func() { p.cmd.Process.Kill() })
	e := p.event(t, "started")
	if !e.OK || e.Root == nil {
		t.Fatal(e)
	}
	p.url = result(t, e)["url"].(string)
	return p
}
func (p *serveProcess) event(t *testing.T, want string) envelope {
	t.Helper()
	select {
	case e, ok := <-p.events:
		if !ok {
			t.Fatal("serve output ended before", want)
		}
		if e.Command != "serve" || e.SchemaVersion != 1 || e.Errors == nil || result(t, e)["event"] != want {
			t.Fatalf("expected %s: %+v", want, e)
		}
		return e
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for", want)
	}
	return envelope{}
}
func (p *serveProcess) stop(t *testing.T) {
	t.Helper()
	p.stopSignal(t, os.Interrupt)
}
func (p *serveProcess) stopSignal(t *testing.T, signal os.Signal) {
	t.Helper()
	if err := p.cmd.Process.Signal(signal); err != nil {
		t.Fatal(err)
	}
	if e := p.event(t, "stopped"); !e.OK {
		t.Fatal(e)
	}
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatal(err, p.stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("process did not stop")
	}
	if p.stderr.Len() != 0 {
		t.Fatal(p.stderr.String())
	}
	if _, ok := <-p.events; ok {
		t.Fatal("extra lifecycle output")
	}
	addr := strings.TrimSuffix(strings.TrimPrefix(p.url, "http://"), "/")
	if c, err := net.DialTimeout("tcp4", addr, time.Second); err == nil {
		c.Close()
		t.Fatal("listener survived shutdown signal", signal)
	}
}
func getServe(t *testing.T, url string, status int) []byte {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != status {
		t.Fatalf("%s: %s %s", url, resp.Status, data)
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal(resp.Header)
	}
	return data
}

func TestServeSelectionAndLifecycle(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	run(t, root, "", 0, "init", "--json")
	run(t, other, "", 0, "init", "--json")
	id := newID(t, run(t, root, "", 0, "new", "Selected project", "--json"))
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, cwd string
		args      []string
	}{
		{"nested", nested, []string{"serve", "--port=0", "--json"}},
		{"root", root, []string{"serve", "--port=0", "--json"}},
		{"project", other, []string{"--project", root, "serve", "--port=0", "--json"}},
		{"config", other, []string{"serve", "--config", filepath.Join(root, ".wrk/config.yaml"), "--port=0", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := startServe(t, binary, tc.cwd, tc.args...)
			data := getServe(t, p.url+"api/items/"+id, 200)
			if !bytes.Contains(data, []byte("Selected project")) || !bytes.Contains(data, []byte(root)) {
				t.Fatal(string(data))
			}
			// Edits through the CLI remain possible while the server is alive.
			run(t, root, "", 0, "update", id, "--status=done", "--json")
			data = getServe(t, p.url+"api/items/"+id, 200)
			if !bytes.Contains(data, []byte(`"status":"done"`)) {
				t.Fatal(string(data))
			}
			getServe(t, p.url+"api/project?project="+other, 400)
			port := strings.TrimSuffix(strings.TrimPrefix(p.url, "http://127.0.0.1:"), "/")
			e := run(t, root, "", 1, "serve", "--port="+port, "--json")
			if e.Errors[0].Code != "LISTEN" {
				t.Fatal(e)
			}
			if tc.name == "nested" {
				run(t, nested, "", 0, "init", "--json")
				// New nearer projects cannot change a running server's selection.
				getServe(t, p.url+"api/items/"+id, 200)
			}
			p.stop(t)
		})
	}
	for _, selector := range []string{"--project", "--config"} {
		t.Run("relative-"+selector, func(t *testing.T) {
			target := root
			if selector == "--config" {
				target = filepath.Join(root, ".wrk/config.yaml")
			}
			relative, err := filepath.Rel(other, target)
			if err != nil {
				t.Fatal(err)
			}
			p := startServe(t, binary, other, "serve", selector, relative, "--port=0", "--json")
			if data := getServe(t, p.url+"api/items/"+id, 200); !bytes.Contains(data, []byte(root)) {
				t.Fatal(string(data))
			}
			p.stopSignal(t, syscall.SIGTERM)
		})
	}
}

func TestServeDefaultAndRequestedPorts(t *testing.T) {
	root := t.TempDir()
	run(t, root, "", 0, "init", "--json")
	for _, address := range []string{"127.0.0.1:7331", "127.0.0.1:0"} {
		t.Run(address, func(t *testing.T) {
			listener, err := net.Listen("tcp4", address)
			if err != nil {
				if address == "127.0.0.1:7331" {
					t.Skip("default port already occupied:", err)
				}
				t.Fatal(err)
			}
			defer listener.Close()
			actual := listener.Addr().String()
			args := []string{"serve", "--json"}
			if address != "127.0.0.1:7331" {
				_, port, err := net.SplitHostPort(actual)
				if err != nil {
					t.Fatal(err)
				}
				args = append(args, "--port="+port)
			}
			if e := run(t, root, "", 1, args...); e.Errors[0].Code != "LISTEN" {
				t.Fatal(e)
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			p := startServe(t, binary, root, args...)
			if p.url != "http://"+actual+"/" {
				t.Fatal("port changed", p.url, actual)
			}
			getServe(t, p.url+"api/project", 200)
			p.stopSignal(t, syscall.SIGTERM)
		})
	}
}

func TestServeOpenExecutable(t *testing.T) {
	root, helpers := t.TempDir(), t.TempDir()
	run(t, root, "", 0, "init", "--json")
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	// Use a controlled launcher on PATH, never a developer's desktop browser.
	launcher := filepath.Join(helpers, name)
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			script := "#!/bin/sh\n[ \"$#\" = 1 ] || exit 2\nprintf '%s' \"$1\" > opened-url\n"
			if fail {
				script += "exit 1\n"
			}
			if err := os.WriteFile(launcher, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			p := startServeEnv(t, binary, root, []string{"PATH=" + helpers}, "serve", "--port=0", "--open", "--json")
			if fail {
				e := p.event(t, "warning")
				warning := result(t, e)["warning"].(map[string]any)
				if !e.OK || warning["code"] != "OPEN_BROWSER" {
					t.Fatal(e)
				}
			}
			deadline := time.Now().Add(3 * time.Second)
			for {
				data, err := os.ReadFile(filepath.Join(root, "opened-url"))
				if err == nil && string(data) == p.url {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("launcher did not receive the actual URL", string(data), err)
				}
				time.Sleep(10 * time.Millisecond)
			}
			getServe(t, p.url+"api/project", 200)
			p.stop(t)
			if err := os.Remove(filepath.Join(root, "opened-url")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestServeStartupErrors(t *testing.T) {
	root := t.TempDir()
	for _, port := range []string{"-1", "65536", "bad", ""} {
		e := run(t, root, "", 2, "serve", "--port="+port, "--json")
		if e.Errors[0].Code != "USAGE" {
			t.Fatal(e)
		}
	}
	run(t, root, "", 1, "serve", "--port=0", "--json")
	run(t, root, "", 1, "serve", "--project=missing", "--port=0", "--json")
	if _, err := os.Stat(filepath.Join(root, ".wrk")); !os.IsNotExist(err) {
		t.Fatal("serve initialized missing project", err)
	}
	run(t, root, "", 0, "init", "--json")
	put(t, root, ".wrk/wrk-deadbeef.md", "invalid")
	e := run(t, root, "", 1, "serve", "--port=0", "--json")
	if len(e.Errors) < 2 {
		t.Fatal(e)
	}
}

func TestPackagedBinaryServesEmbeddedAssets(t *testing.T) {
	// Reproduce the one-executable archive contract, extract away from the checkout,
	// and run with no asset directory or frontend runtime alongside the executable.
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "wrk", Mode: 0755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(&archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tr := tar.NewReader(reader)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	executable := filepath.Join(dest, hdr.Name)
	data, err = io.ReadAll(tr)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, data, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatal("extra archive entry", err)
	}
	root, cwd := t.TempDir(), t.TempDir()
	run(t, root, "", 0, "init", "--json")
	p := startServe(t, executable, cwd, "serve", "--project", root, "--port=0", "--json")
	html := getServe(t, p.url, 200)
	refs := regexp.MustCompile(`(?:<script[^>]*src|<link[^>]*href)="([^"]+)"`).FindAllSubmatch(html, -1)
	if len(refs) != 2 {
		t.Fatal("expected embedded CSS and JS", string(html))
	}
	for _, ref := range refs {
		path := string(ref[1])
		if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
			t.Fatal("external runtime asset", path)
		}
		asset := getServe(t, p.url+strings.TrimPrefix(path, "/"), 200)
		if len(asset) == 0 {
			t.Fatal("empty asset", path)
		}
	}
	// The bundled app's local module must also work outside the checkout.
	module := getServe(t, p.url+"model.mjs", 200)
	if !bytes.Contains(module, []byte("export function selectItems")) {
		t.Fatal("missing browser model module")
	}
	live := getServe(t, p.url+"live.mjs", 200)
	if !bytes.Contains(live, []byte("export function createPoller")) {
		t.Fatal("missing browser live module")
	}
	editor := getServe(t, p.url+"editor.mjs", 200)
	if !bytes.Contains(editor, []byte("export function createEditor")) {
		t.Fatal("missing browser editing module")
	}
	detail := getServe(t, p.url+"detail.mjs", 200)
	if !bytes.Contains(detail, []byte("export function createDetailEditor")) {
		t.Fatal("missing browser detail module")
	}
	inline := getServe(t, p.url+"inline.mjs", 200)
	if !bytes.Contains(inline, []byte("export function createInlineChildren")) {
		t.Fatal("missing browser inline creation module")
	}
	pickers := getServe(t, p.url+"pickers.mjs", 200)
	if !bytes.Contains(pickers, []byte("export function createItemPicker")) {
		t.Fatal("missing browser picker module")
	}
	if !bytes.Contains(html, []byte("Local workspace")) {
		t.Fatal(string(html))
	}
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
	p.stop(t)
}

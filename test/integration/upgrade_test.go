package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Build the real CLI twice with a Go overlay that changes ONLY the HTTP transport
// and a directory-sync failure seam. No fixture URLs, flags, or trust bypasses are
// present in production builds. Requests still use the fixed GitHub URLs and TLS.
func TestReleaseUpgradeEndToEnd(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	archiveName := fmt.Sprintf("wrk_1.1.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	manifestName := "wrk_1.1.0_checksums.txt"
	var archive, manifest []byte
	var mode atomic.Int32 // 1 truncated, 2 wrong checksum, 3 wait for concurrency/kill
	var waiting, release chan struct{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/calebmchenry/wrk/releases/latest":
			assets := []map[string]string{}
			for _, name := range []string{archiveName, manifestName} {
				assets = append(assets, map[string]string{"name": name, "browser_download_url": "https://github.com/calebmchenry/wrk/releases/download/v1.1.0/" + name})
			}
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.1.0", "draft": false, "prerelease": false, "assets": assets})
		case strings.HasSuffix(r.URL.Path, manifestName):
			if mode.Load() == 2 {
				fmt.Fprintf(w, "%s  %s\n", strings.Repeat("0", 64), archiveName)
			} else {
				w.Write(manifest)
			}
		case strings.HasSuffix(r.URL.Path, archiveName):
			if mode.Load() == 3 {
				close(waiting)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(archive)))
			if mode.Load() == 1 {
				w.Write(archive[:len(archive)/2])
			} else {
				w.Write(archive)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	clientPath := filepath.Join(root, "internal/upgrade/client.go")
	source, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(source), "import (", "import (\n\"crypto/tls\"\n\"crypto/x509\"\n\"net\"", 1)
	needle := "&http.Client{Timeout: 30 * time.Second}"
	if !strings.Contains(text, needle) {
		t.Fatal("update transport overlay for Client constructor")
	}
	text = strings.Replace(text, needle, "fixtureHTTP()", 1)
	text = strings.Replace(text, "Executable: os.Executable}", "Executable: os.Executable, syncDir: fixtureSync}", 1)
	text += fmt.Sprintf(`
func fixtureHTTP() *http.Client {
 pool := x509.NewCertPool()
 if !pool.AppendCertsFromPEM([]byte(%q)) { panic("bad fixture certificate") }
 return &http.Client{Timeout: 10*time.Second, Transport: &http.Transport{
  TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "example.com"},
  DialContext: func(ctx context.Context, network, address string)(net.Conn,error){return (&net.Dialer{}).DialContext(ctx,network,%q)},
 }}
}
func fixtureSync(f *os.File) error {
 if os.Getenv("WRK_TEST_SYNC_FAILURE") == "1" { return fmt.Errorf("injected directory sync failure") }
 return f.Sync()
}
`, cert, server.Listener.Addr().String())
	replacement := filepath.Join(work, "client.go")
	os.WriteFile(replacement, []byte(text), 0600)
	overlay := filepath.Join(work, "overlay.json")
	data, _ := json.Marshal(map[string]any{"Replace": map[string]string{clientPath: replacement}})
	os.WriteFile(overlay, data, 0600)
	build := func(version, commit string) []byte {
		t.Helper()
		path := filepath.Join(work, "wrk-"+version)
		cmd := exec.Command("go", "build", "-overlay", overlay, "-ldflags", "-X wrk/internal/buildinfo.Version="+version+" -X wrk/internal/buildinfo.Commit="+commit+" -X wrk/internal/buildinfo.Kind=release", "-o", path, "./cmd/wrk")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", version, err, out)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	old := build("1.0.0", strings.Repeat("a", 40))
	next := build("1.1.0", strings.Repeat("b", 40))
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "wrk", Mode: 0755, Size: int64(len(next))}); err != nil {
		t.Fatal(err)
	}
	tw.Write(next)
	tw.Close()
	gz.Close()
	archive = buf.Bytes()
	sum := sha256.Sum256(archive)
	manifest = []byte(fmt.Sprintf("%x  %s\n", sum, archiveName))
	install := func() (string, string) {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "wrk")
		if err := os.WriteFile(path, old, 0755); err != nil {
			t.Fatal(err)
		}
		return dir, path
	}
	invoke := func(path, cwd string, want int, extraEnv []string, args ...string) envelope {
		t.Helper()
		cmd := exec.Command(path, args...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), extraEnv...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		exit := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				exit = e.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if exit != want || stderr.Len() != 0 {
			t.Fatalf("%v: exit %d want %d: %s %s", args, exit, want, stdout.String(), stderr.String())
		}
		var e envelope
		if err := json.Unmarshal(stdout.Bytes(), &e); err != nil || e.Root != nil || e.OK != (want == 0) {
			t.Fatalf("%s %v", stdout.String(), err)
		}
		return e
	}
	assertVersion := func(path, cwd, want string) {
		t.Helper()
		e := invoke(path, cwd, 0, nil, "version", "--json")
		if result(t, e)["version"] != want {
			t.Fatal(string(e.Result))
		}
	}
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("complete-invalid-project-%t", invalid), func(t *testing.T) {
			dir, path := install()
			if invalid {
				os.Mkdir(filepath.Join(dir, ".wrk"), 0755)
				os.WriteFile(filepath.Join(dir, ".wrk/config.yaml"), []byte("version: 999\n"), 0600)
			}
			before := inventory(t, dir)
			check := invoke(path, dir, 0, nil, "upgrade", "--check", "--json")
			if result(t, check)["available"] != true {
				t.Fatal(string(check.Result))
			}
			after := inventory(t, dir)
			if fmt.Sprint(before) != fmt.Sprint(after) {
				t.Fatal("check wrote files")
			}
			assertVersion(path, dir, "1.0.0")
			up := invoke(path, dir, 0, nil, "upgrade", "--json")
			if result(t, up)["changed"] != true {
				t.Fatal(string(up.Result))
			}
			assertVersion(path, dir, "1.1.0")
			up = invoke(path, dir, 0, nil, "upgrade", "--json")
			if result(t, up)["changed"] != false || result(t, up)["available"] != false {
				t.Fatal(string(up.Result))
			}
		})
	}
	for _, m := range []int32{1, 2} {
		t.Run(fmt.Sprint("download-failure-", m), func(t *testing.T) {
			mode.Store(m)
			defer mode.Store(0)
			dir, path := install()
			e := invoke(path, dir, 1, nil, "upgrade", "--json")
			if string(e.Result) != "null" {
				t.Fatal(string(e.Result))
			}
			assertVersion(path, dir, "1.0.0")
			data, _ := os.ReadFile(path)
			if !bytes.Equal(data, old) {
				t.Fatal("old binary changed")
			}
			stages, _ := filepath.Glob(filepath.Join(dir, ".wrk-upgrade-*"))
			if len(stages) != 0 {
				t.Fatal(stages)
			}
		})
	}
	t.Run("post-publication", func(t *testing.T) {
		dir, path := install()
		e := invoke(path, dir, 1, []string{"WRK_TEST_SYNC_FAILURE=1"}, "upgrade", "--json")
		r := result(t, e)
		if r["changed"] != true || r["publication"] != "committed" || len(e.Errors) != 1 || e.Errors[0].Code != "DURABILITY_UNCERTAIN" {
			t.Fatal(string(e.Result), e.Errors)
		}
		assertVersion(path, dir, "1.1.0")
	})
	for _, kill := range []bool{false, true} {
		t.Run(fmt.Sprintf("concurrent-kill-%t", kill), func(t *testing.T) {
			waiting, release = make(chan struct{}), make(chan struct{})
			mode.Store(3)
			defer mode.Store(0)
			dir, path := install()
			cmd := exec.Command(path, "upgrade", "--json")
			cmd.Dir = dir
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-waiting:
			case <-time.After(15 * time.Second):
				cmd.Process.Kill()
				cmd.Wait()
				t.Fatalf("no download started: %s", output.String())
			}
			if kill {
				cmd.Process.Kill()
				cmd.Wait()
				close(release)
				assertVersion(path, dir, "1.0.0")
			} else {
				e := invoke(path, dir, 1, nil, "upgrade", "--json")
				if len(e.Errors) != 1 || e.Errors[0].Code != "BUSY" {
					t.Fatal(e.Errors)
				}
				close(release)
				if err := cmd.Wait(); err != nil {
					t.Fatalf("%v %s", err, output.String())
				}
				assertVersion(path, dir, "1.1.0")
			}
		})
	}
	t.Logf("real executable replacement verified on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func TestDevelopmentVersionAndInstallGuidance(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"version", "--json"}, {"--version", "--json"}} {
		e := run(t, root, "", 0, args...)
		r := result(t, e)
		if r["version"] != "dev" || r["build_kind"] != "development" || e.Root != nil {
			t.Fatal(r)
		}
	}
	e := run(t, root, "", 1, "upgrade", "--json")
	if len(e.Errors) != 1 || e.Errors[0].Code != "INSTALL_METHOD" {
		t.Fatal(e.Errors)
	}
}

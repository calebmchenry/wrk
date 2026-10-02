package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wrk/internal/buildinfo"
	"wrk/internal/upgrade"
)

type versionTransport func(*http.Request) (*http.Response, error)

func (f versionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProjectIndependentVersionAndUpgrade(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".wrk"), 0755)
	os.WriteFile(filepath.Join(root, ".wrk", "config.yaml"), []byte("version: 999\n"), 0600)
	i := buildinfo.Info{Version: "1.0.0", Commit: strings.Repeat("a", 40), BuildKind: "release", GOOS: "linux", GOARCH: "amd64"}
	c := upgrade.NewClient()
	calls := 0
	c.HTTP.Transport = versionTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"tag_name":"v1.1.0","draft":false,"prerelease":false,"assets":[{"name":"wrk_1.1.0_linux_amd64.tar.gz","browser_download_url":"https://github.com/calebmchenry/wrk/releases/download/v1.1.0/wrk_1.1.0_linux_amd64.tar.gz"},{"name":"wrk_1.1.0_checksums.txt","browser_download_url":"https://github.com/calebmchenry/wrk/releases/download/v1.1.0/wrk_1.1.0_checksums.txt"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	for _, cwd := range []string{root, filepath.Join(root, "does-not-exist")} {
		for _, args := range [][]string{{"version", "--json"}, {"--json", "--version"}, {"upgrade", "--check", "--json"}, {"help", "upgrade", "--json"}} {
			var out, errout bytes.Buffer
			if exit := run(args, cwd, strings.NewReader(""), &out, &errout, c, i); exit != 0 || errout.Len() != 0 {
				t.Fatalf("%v: %d %s %s", args, exit, out.String(), errout.String())
			}
			var e Envelope
			if err := json.Unmarshal(out.Bytes(), &e); err != nil || e.ProjectRoot != nil || !e.OK {
				t.Fatal(e, err)
			}
		}
	}
	if calls != 2 {
		t.Fatalf("non-upgrade command used network: %d", calls)
	}
	files, _ := os.ReadDir(filepath.Join(root, ".wrk"))
	if len(files) != 1 {
		t.Fatal("check changed project")
	}
	for _, args := range [][]string{{"version", "extra"}, {"--version=1"}, {"upgrade", "--check", "--check"}, {"upgrade", "--force"}, {"upgrade", "v1.0.0"}, {"version", "--check"}, {"update", "--check"}} {
		if _, err := Parse(args); err == nil {
			t.Fatal(args)
		}
	}
	var out, errout bytes.Buffer
	if run([]string{"version"}, root, nil, &out, &errout, c, i) != 0 || !strings.Contains(out.String(), "wrk 1.0.0 (release, commit") {
		t.Fatal(out.String())
	}
	c.HTTP.Transport = versionTransport(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
	out.Reset()
	errout.Reset()
	if run([]string{"upgrade", "--check", "--json"}, root, nil, &out, &errout, c, i) != 1 || errout.Len() != 0 || !strings.Contains(out.String(), `"result":null`) {
		t.Fatal(out.String(), errout.String())
	}
}

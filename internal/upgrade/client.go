// Package upgrade checks and installs releases independently of any wrk project.
package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"wrk/internal/buildinfo"
)

const latestURL = "https://api.github.com/repos/calebmchenry/wrk/releases/latest"
const maxMetadata = 2 << 20
const maxManifest = 1 << 20
const maxArchive = 64 << 20
const maxExecutable = 128 << 20

type Error struct{ Code, Message string }

func (e *Error) Error() string                    { return e.Message }
func fail(code, format string, args ...any) error { return &Error{code, fmt.Sprintf(format, args...)} }

type Client struct {
	HTTP       *http.Client
	LatestURL  string
	Executable func() (string, error)
	// Filesystem/probe seams are private and only overridden by failure tests.
	probe         func(context.Context, string) (buildinfo.Info, error)
	beforePublish func()
	rename        func(string, string) error
	syncDir       func(*os.File) error
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}, LatestURL: latestURL, Executable: os.Executable}
}

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}
type release struct {
	Tag        string  `json:"tag_name"`
	Draft      *bool   `json:"draft"`
	Prerelease *bool   `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}
type Result struct {
	CurrentVersion    string `json:"current_version"`
	LatestVersion     string `json:"latest_version"`
	Available         *bool  `json:"available"`
	Reason            string `json:"reason,omitempty"`
	Changed           bool   `json:"changed"`
	Destination       string `json:"destination,omitempty"`
	Publication       string `json:"publication,omitempty"`
	archive, manifest Asset
}

func validHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}

func (c *Client) get(ctx context.Context, raw string, limit int64) ([]byte, error) {
	if !validHTTPS(raw) {
		return nil, fail("RELEASE_INVALID", "release URLs must use HTTPS without credentials")
	}
	// Always enforce a deadline and redirect policy, including with an injected transport.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	h := *c.HTTP
	h.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !validHTTPS(req.URL.String()) {
			return fmt.Errorf("unsafe or excessive release redirects")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, fail("RELEASE_INVALID", "invalid release URL")
	}
	req.Header.Set("User-Agent", "wrk-upgrade")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := h.Do(req)
	if err != nil {
		return nil, fail("NETWORK", "release request failed (check connectivity and retry): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fail("RELEASE_NOT_FOUND", "release or asset is unavailable; install manually from https://github.com/calebmchenry/wrk/releases")
	}
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return nil, fail("RATE_LIMIT", "release service denied the request (HTTP %d); wait and retry, or download manually", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fail("NETWORK", "release service returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return nil, fail("RELEASE_INVALID", "release response exceeds the %d-byte limit", limit)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fail("NETWORK", "incomplete release download: %v", err)
	}
	if int64(len(data)) > limit {
		return nil, fail("RELEASE_INVALID", "release response exceeds the %d-byte limit", limit)
	}
	return data, nil
}

func (c *Client) Check(ctx context.Context, current buildinfo.Info) (Result, error) {
	r := Result{CurrentVersion: current.Version}
	// Reject unsupported platforms even when the server is unavailable.
	if _, _, err := buildinfo.AssetNames("0.0.0", current.GOOS, current.GOARCH); err != nil {
		return r, fail("UNSUPPORTED_PLATFORM", "%v", err)
	}
	data, err := c.get(ctx, c.LatestURL, maxMetadata)
	if err != nil {
		return r, err
	}
	var rel release
	if err := json.Unmarshal(data, &rel); err != nil {
		return r, fail("RELEASE_INVALID", "malformed release metadata: %v", err)
	}
	if rel.Draft == nil || rel.Prerelease == nil || *rel.Draft || *rel.Prerelease {
		return r, fail("RELEASE_INVALID", "latest release must be a published stable release")
	}
	v, err := buildinfo.FromTag(rel.Tag)
	if err != nil {
		return r, fail("RELEASE_INVALID", "%v", err)
	}
	archive, manifest, _ := buildinfo.AssetNames(v, current.GOOS, current.GOARCH)
	resolve := func(name string) (Asset, error) {
		var matches []Asset
		for _, a := range rel.Assets {
			if a.Name == name {
				matches = append(matches, a)
			}
		}
		if len(matches) != 1 {
			return Asset{}, fail("RELEASE_INVALID", "expected exactly one release asset %s, found %d", name, len(matches))
		}
		a := matches[0]
		// Initial asset URLs must belong to this exact repository, tag, and name.
		expected := "https://github.com/calebmchenry/wrk/releases/download/" + rel.Tag + "/" + name
		if a.URL != expected {
			return Asset{}, fail("RELEASE_INVALID", "unexpected download URL for %s", name)
		}
		return a, nil
	}
	if r.archive, err = resolve(archive); err != nil {
		return r, err
	}
	if r.manifest, err = resolve(manifest); err != nil {
		return r, err
	}
	r.LatestVersion = v
	if !current.IsRelease() {
		r.Reason = "current build has no comparable stable release metadata; install an official release manually"
		return r, nil
	}
	available := buildinfo.Compare(current.Version, v) < 0
	r.Available = &available
	if !available {
		r.Reason = "already current or newer; no downgrade"
	}
	return r, nil
}

func (r Result) Human() string {
	text := fmt.Sprintf("wrk %s; latest stable %s", r.CurrentVersion, r.LatestVersion)
	if r.Changed {
		text += "; installed at " + r.Destination
	} else if r.Available != nil && *r.Available {
		text += "; update available"
	}
	if r.Reason != "" {
		text += "; " + r.Reason
	}
	return strings.TrimSpace(text)
}

package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"wrk/internal/buildinfo"
)

const testCommit = "0123456789012345678901234567890123456789"

func info(v string) buildinfo.Info {
	return buildinfo.Info{Version: v, Commit: testCommit, BuildKind: "release", GOOS: "linux", GOARCH: "amd64"}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixtureRelease(v string) release {
	f := false
	a, m, _ := buildinfo.AssetNames(v, "linux", "amd64")
	rel := release{Tag: "v" + v, Draft: &f, Prerelease: &f}
	for _, name := range []string{a, m} {
		rel.Assets = append(rel.Assets, Asset{Name: name, URL: "https://github.com/calebmchenry/wrk/releases/download/v" + v + "/" + name})
	}
	return rel
}
func response(data []byte) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Header: make(http.Header)}
}
func fixtureClient(rel release) *Client {
	c := NewClient()
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		data, _ := json.Marshal(rel)
		return response(data), nil
	})
	return c
}
func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestCheckOrderingAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		current   string
		available bool
	}{{"1.2.0", true}, {"1.10.0", false}, {"2.0.0", false}} {
		r, err := fixtureClient(fixtureRelease("1.10.0")).Check(context.Background(), info(tc.current))
		if err != nil || r.Available == nil || *r.Available != tc.available || r.LatestVersion != "1.10.0" || r.Changed {
			t.Fatalf("%+v %v", r, err)
		}
	}
	for _, kind := range []string{"development", "snapshot", "homebrew", "release"} {
		i := info("dev")
		i.BuildKind = kind
		r, err := fixtureClient(fixtureRelease("1.0.0")).Check(context.Background(), i)
		if err != nil || r.Available != nil || r.Reason == "" {
			t.Fatalf("%+v %v", r, err)
		}
	}
}

func TestCheckRejectsMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*release)
	}{
		{"draft", func(r *release) { v := true; r.Draft = &v }},
		{"prerelease", func(r *release) { v := true; r.Prerelease = &v }},
		{"missing flags", func(r *release) { r.Draft = nil }},
		{"bad tag", func(r *release) { r.Tag = "v1.0.0-rc.1" }},
		{"missing archive", func(r *release) { r.Assets = r.Assets[1:] }},
		{"missing manifest", func(r *release) { r.Assets = r.Assets[:1] }},
		{"duplicate", func(r *release) { r.Assets = append(r.Assets, r.Assets[0]) }},
		{"http", func(r *release) { r.Assets[0].URL = strings.Replace(r.Assets[0].URL, "https:", "http:", 1) }},
		{"other release", func(r *release) { r.Assets[0].URL = strings.Replace(r.Assets[0].URL, "v1.0.0", "v2.0.0", 1) }},
		{"other repository", func(r *release) { r.Assets[0].URL = strings.Replace(r.Assets[0].URL, "calebmchenry", "elsewhere", 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixtureRelease("1.0.0")
			tc.change(&r)
			_, err := fixtureClient(r).Check(context.Background(), info("0.0.1"))
			if code(err) != "RELEASE_INVALID" {
				t.Fatal(err)
			}
		})
	}
	i := info("1.0.0")
	i.GOOS = "windows"
	if _, err := fixtureClient(fixtureRelease("1.0.0")).Check(context.Background(), i); code(err) != "UNSUPPORTED_PLATFORM" {
		t.Fatal(err)
	}
}

func TestHTTPFailuresAndBounds(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   string
	}{{404, "RELEASE_NOT_FOUND"}, {403, "RATE_LIMIT"}, {429, "RATE_LIMIT"}, {500, "NETWORK"}} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			c := NewClient()
			c.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
				r := response(nil)
				r.StatusCode = tc.status
				return r, nil
			})
			_, err := c.Check(context.Background(), info("1.0.0"))
			if code(err) != tc.want {
				t.Fatal(err)
			}
		})
	}
	for _, data := range [][]byte{[]byte("null"), []byte("{}"), []byte("{"), []byte("{} {}"), bytes.Repeat([]byte("a"), maxMetadata+1)} {
		c := NewClient()
		c.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response(data), nil })
		if _, err := c.Check(context.Background(), info("1.0.0")); code(err) != "RELEASE_INVALID" {
			t.Fatal(err)
		}
	}
	c := NewClient()
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := c.Check(ctx, info("1.0.0")); code(err) != "NETWORK" {
		t.Fatal(err)
	}
	c.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	if _, err := c.Check(context.Background(), info("1.0.0")); code(err) != "NETWORK" {
		t.Fatal(err)
	}
	c.HTTP.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		r := response(nil)
		r.StatusCode = 302
		r.Header.Set("Location", "http://example.com/unsafe")
		return r, nil
	})
	if _, err := c.Check(context.Background(), info("1.0.0")); code(err) != "NETWORK" {
		t.Fatal(err)
	}
}

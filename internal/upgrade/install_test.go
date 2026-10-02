package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wrk/internal/buildinfo"
)

func archiveOf(t *testing.T, headers []*tar.Header, contents [][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for n, h := range headers {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if n < len(contents) {
			if _, err := tw.Write(contents[n]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func executableArchive(t *testing.T, data []byte) []byte {
	return archiveOf(t, []*tar.Header{{Name: "wrk", Mode: 0755, Size: int64(len(data)), Typeflag: tar.TypeReg}}, [][]byte{data})
}

func TestArchiveAndChecksumRejection(t *testing.T) {
	good := executableArchive(t, []byte("candidate"))
	sum := sha256.Sum256(good)
	manifest := fmt.Sprintf("%x  asset.tar.gz\n", sum)
	if err := verifyChecksum([]byte(manifest), good, "asset.tar.gz"); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"", "garbage", manifest + manifest, strings.Replace(manifest, "asset.tar.gz", "other", 1), strings.Repeat("0", 64) + "  asset.tar.gz\n"} {
		if err := verifyChecksum([]byte(m), good, "asset.tar.gz"); code(err) != "INTEGRITY" {
			t.Fatal(err)
		}
	}
	for _, h := range []*tar.Header{
		{Name: "../wrk", Mode: 0755, Size: 1}, {Name: "/wrk", Mode: 0755, Size: 1}, {Name: "dir/wrk", Mode: 0755, Size: 1},
		{Name: "wrk", Typeflag: tar.TypeSymlink, Linkname: "elsewhere"}, {Name: "wrk", Typeflag: tar.TypeLink, Linkname: "elsewhere"}, {Name: "wrk", Typeflag: tar.TypeDir},
	} {
		t.Run(h.Name+fmt.Sprint(h.Typeflag), func(t *testing.T) {
			content := []byte(nil)
			if h.Size > 0 {
				content = []byte("x")
			}
			a := archiveOf(t, []*tar.Header{h}, [][]byte{content})
			if err := extract(a, io.Discard); code(err) != "INTEGRITY" {
				t.Fatal(err)
			}
		})
	}
	duplicate := archiveOf(t, []*tar.Header{{Name: "wrk", Mode: 0755, Size: 1}, {Name: "wrk", Mode: 0755, Size: 1}}, [][]byte{[]byte("a"), []byte("b")})
	corrupt := append([]byte(nil), good...)
	corrupt[len(corrupt)-8] ^= 1
	for _, a := range [][]byte{duplicate, good[:len(good)-4], corrupt, []byte("bad")} {
		if err := extract(a, io.Discard); code(err) != "INTEGRITY" {
			t.Fatal(err)
		}
	}
	var expanded bytes.Buffer
	gz := gzip.NewWriter(&expanded)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "wrk", Size: maxExecutable + 1}); err != nil {
		t.Fatal(err)
	}
	// Deliberately incomplete oversized payload: reject the header before reading it.
	gz.Close()
	if err := extract(expanded.Bytes(), io.Discard); code(err) != "INTEGRITY" {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := extract(good, &out); err != nil || out.String() != "candidate" {
		t.Fatalf("%s %v", out.String(), err)
	}
}

func installer(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "wrk")
	if err := os.WriteFile(target, []byte("old executable"), 0755); err != nil {
		t.Fatal(err)
	}
	target, _ = filepath.EvalSymlinks(target)
	archive := executableArchive(t, []byte("new executable"))
	rel := fixtureRelease("1.1.0")
	meta, _ := json.Marshal(rel)
	sum := sha256.Sum256(archive)
	manifest := []byte(fmt.Sprintf("%x  %s\n", sum, rel.Assets[0].Name))
	c := NewClient()
	c.Executable = func() (string, error) { return target, nil }
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case latestURL:
			return response(meta), nil
		case rel.Assets[0].URL:
			return response(archive), nil
		case rel.Assets[1].URL:
			return response(manifest), nil
		default:
			return nil, fmt.Errorf("unexpected URL %s", r.URL)
		}
	})
	c.probe = func(ctx context.Context, path string) (buildinfo.Info, error) {
		if path == target {
			return info("1.0.0"), nil
		}
		return info("1.1.0"), nil
	}
	return c, target
}
func assertOldAndClean(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "old executable" {
		t.Fatalf("old binary lost: %s %v", data, err)
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".wrk-upgrade-*"))
	if len(files) > 0 {
		t.Fatalf("stages left: %v", files)
	}
}

func TestInstallAndPublicationFailure(t *testing.T) {
	for _, syncFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(syncFailure), func(t *testing.T) {
			c, path := installer(t)
			if syncFailure {
				c.syncDir = func(*os.File) error { return errors.New("sync failed") }
			}
			r, err := c.Install(context.Background(), info("1.0.0"))
			if !r.Changed || r.Publication != "committed" || r.Destination != path || (syncFailure && code(err) != "DURABILITY_UNCERTAIN") || (!syncFailure && err != nil) {
				t.Fatalf("%+v %v", r, err)
			}
			data, _ := os.ReadFile(path)
			if string(data) != "new executable" {
				t.Fatal(string(data))
			}
			st, _ := os.Stat(path)
			if st.Mode().Perm() != 0755 {
				t.Fatal(st.Mode())
			}
		})
	}
}

func TestInstallFailuresLeaveOldExecutable(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		setup      func(*Client, string)
	}{
		{"checksum", "INTEGRITY", func(c *Client, _ string) {
			transport := c.HTTP.Transport
			c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "checksums.txt") {
					return response([]byte("bad manifest")), nil
				}
				return transport.RoundTrip(r)
			})
		}},
		{"download", "NETWORK", func(c *Client, _ string) {
			transport := c.HTTP.Transport
			c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, ".tar.gz") {
					return nil, errors.New("connection lost")
				}
				return transport.RoundTrip(r)
			})
		}},
		{"candidate version", "CANDIDATE_INVALID", func(c *Client, p string) {
			c.probe = func(_ context.Context, path string) (buildinfo.Info, error) {
				if path == p {
					return info("1.0.0"), nil
				}
				return info("1.2.0"), nil
			}
		}},
		{"candidate probe", "CANDIDATE_INVALID", func(c *Client, p string) {
			c.probe = func(_ context.Context, path string) (buildinfo.Info, error) {
				if path == p {
					return info("1.0.0"), nil
				}
				return buildinfo.Info{}, fail("CANDIDATE_INVALID", "execution failed")
			}
		}},
		{"stale process", "CONFLICT", func(c *Client, _ string) {
			c.probe = func(context.Context, string) (buildinfo.Info, error) { return info("2.0.0"), nil }
		}},
		{"rename permission", "PERMISSION", func(c *Client, _ string) { c.rename = func(string, string) error { return os.ErrPermission } }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, path := installer(t)
			tc.setup(c, path)
			r, err := c.Install(context.Background(), info("1.0.0"))
			if code(err) != tc.want || r.Changed || r.Publication != "" {
				t.Fatalf("%+v %v", r, err)
			}
			assertOldAndClean(t, path)
		})
	}
}

func TestSymlinkConcurrencyAndPermissions(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		c, path := installer(t)
		link := filepath.Join(filepath.Dir(path), "link")
		os.Symlink("wrk", link)
		c.Executable = func() (string, error) { return link, nil }
		r, err := c.Install(context.Background(), info("1.0.0"))
		if err != nil || !r.Changed {
			t.Fatal(r, err)
		}
		st, _ := os.Lstat(link)
		if st.Mode()&os.ModeSymlink == 0 {
			t.Fatal("symlink replaced")
		}
	})
	t.Run("broken symlink", func(t *testing.T) {
		c, path := installer(t)
		link := path + "-link"
		os.Symlink("missing", link)
		c.Executable = func() (string, error) { return link, nil }
		if _, err := c.Install(context.Background(), info("1.0.0")); err == nil {
			t.Fatal("accepted broken symlink")
		}
		assertOldAndClean(t, path)
	})
	t.Run("changed symlink", func(t *testing.T) {
		c, path := installer(t)
		link := path + "-link"
		os.Symlink(path, link)
		c.Executable = func() (string, error) { return link, nil }
		c.beforePublish = func() { os.Remove(link); os.Symlink("missing", link) }
		if _, err := c.Install(context.Background(), info("1.0.0")); code(err) != "CONFLICT" {
			t.Fatal(err)
		}
		assertOldAndClean(t, path)
	})
	t.Run("concurrent writer", func(t *testing.T) {
		c, path := installer(t)
		lock, err := acquire(filepath.Join(filepath.Dir(path), ".wrk.upgrade.lock"))
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if _, err := c.Install(context.Background(), info("1.0.0")); code(err) != "BUSY" {
			t.Fatal(err)
		}
		assertOldAndClean(t, path)
	})
	t.Run("changed target", func(t *testing.T) {
		c, path := installer(t)
		c.beforePublish = func() { os.Rename(path, path+".previous"); os.WriteFile(path, []byte("concurrent version"), 0755) }
		if _, err := c.Install(context.Background(), info("1.0.0")); code(err) != "CONFLICT" {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "concurrent version" {
			t.Fatal("overwrote concurrent install")
		}
		assertOldAndClean(t, path+".previous")
	})
	t.Run("unwritable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		c, path := installer(t)
		dir := filepath.Dir(path)
		os.Chmod(dir, 0555)
		defer os.Chmod(dir, 0755)
		if _, err := c.Install(context.Background(), info("1.0.0")); code(err) != "PERMISSION" {
			t.Fatal(err)
		}
		assertOldAndClean(t, path)
	})
	t.Run("lock symlink", func(t *testing.T) {
		c, path := installer(t)
		os.Symlink(path, filepath.Join(filepath.Dir(path), ".wrk.upgrade.lock"))
		if _, err := c.Install(context.Background(), info("1.0.0")); err == nil {
			t.Fatal("followed lock symlink")
		}
		assertOldAndClean(t, path)
	})
}

func TestNoOpAndInstallationPolicy(t *testing.T) {
	for _, version := range []string{"1.1.0", "2.0.0"} {
		c, path := installer(t)
		r, err := c.Install(context.Background(), info(version))
		if err != nil || r.Changed {
			t.Fatal(r, err)
		}
		assertOldAndClean(t, path)
		entries, _ := os.ReadDir(filepath.Dir(path))
		if len(entries) != 1 {
			t.Fatal("no-op wrote files")
		}
	}
	c, path := installer(t)
	i := info("1.0.0")
	i.BuildKind = "development"
	if _, err := c.Install(context.Background(), i); code(err) != "INSTALL_METHOD" {
		t.Fatal(err)
	}
	assertOldAndClean(t, path)
	for _, p := range []string{"/opt/homebrew/Cellar/wrk/1/bin/wrk", "/usr/local/Cellar/wrk/bin/wrk", "/nix/store/hash-wrk/bin/wrk", "/usr/bin/wrk"} {
		if !packageManaged(p) {
			t.Fatal(p)
		}
	}
	if packageManaged("/usr/local/bin/wrk") {
		t.Fatal("standalone prefix rejected")
	}
}

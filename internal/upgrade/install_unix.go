//go:build darwin || linux

package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"wrk/internal/buildinfo"

	"golang.org/x/sys/unix"
)

func fsError(action string, err error) error {
	code := "IO"
	if errors.Is(err, os.ErrPermission) {
		code = "PERMISSION"
	}
	return fail(code, "%s: %v; use a user-owned standalone installation with a writable directory", action, err)
}

func packageManaged(path string) bool {
	for _, prefix := range []string{"/opt/homebrew/", "/usr/local/Cellar/", "/usr/local/Caskroom/", "/opt/local/", "/nix/store/", "/snap/", "/var/lib/snapd/", "/usr/bin/", "/bin/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func openRegular(path string, flags int, mode uint32) (*os.File, os.FileInfo, error) {
	fd, err := unix.Open(path, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, mode)
	if err != nil {
		return nil, nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("not a regular file")
	}
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, info, nil
}

func acquire(path string) (*os.File, error) {
	f, info, err := openRegular(path, unix.O_RDWR|unix.O_CREAT, 0600)
	if err != nil {
		return nil, fsError("open upgrade lock", err)
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, fail("BUSY", "another upgrade holds this installation's lock; retry after it finishes")
		}
		return nil, fsError("lock installation", err)
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, current) {
		f.Close()
		return nil, fail("CONFLICT", "upgrade lock identity changed")
	}
	return f, nil
}

type fingerprint struct {
	info   os.FileInfo
	digest [sha256.Size]byte
}

func inspect(path string) (fingerprint, error) {
	f, info, err := openRegular(path, unix.O_RDONLY, 0)
	if err != nil {
		return fingerprint{}, err
	}
	defer f.Close()
	if info.Size() > maxExecutable {
		return fingerprint{}, fmt.Errorf("installed executable is too large")
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, maxExecutable+1)); err != nil {
		return fingerprint{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return fingerprint{info, digest}, nil
}
func (f fingerprint) equal(other fingerprint) bool {
	return os.SameFile(f.info, other.info) && f.info.Mode() == other.info.Mode() && f.info.Size() == other.info.Size() && f.info.ModTime() == other.info.ModTime() && f.digest == other.digest
}

func probe(ctx context.Context, path string) (buildinfo.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "version", "--json")
	cmd.WaitDelay = time.Second
	stdout, stderr := &boundedBuffer{limit: 64 << 10}, &boundedBuffer{limit: 4 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return buildinfo.Info{}, fail("CANDIDATE_INVALID", "executable version verification failed: %v", err)
	}
	var e struct {
		Schema  int               `json:"schema_version"`
		Command string            `json:"command"`
		OK      bool              `json:"ok"`
		Root    *string           `json:"project_root"`
		Result  buildinfo.Info    `json:"result"`
		Errors  []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &e); err != nil || e.Schema != 1 || e.Command != "version" || !e.OK || e.Root != nil || e.Errors == nil || len(e.Errors) != 0 || stderr.Len() != 0 || !e.Result.IsRelease() {
		return buildinfo.Info{}, fail("CANDIDATE_INVALID", "executable did not report valid stable release metadata")
	}
	return e.Result, nil
}

func (c *Client) Install(ctx context.Context, current buildinfo.Info) (result Result, err error) {
	if !current.IsRelease() {
		return result, fail("INSTALL_METHOD", "self-upgrade requires an official standalone stable release; install one manually from https://github.com/calebmchenry/wrk/releases (development, go run, and snapshot builds cannot replace themselves)")
	}
	source, err := c.Executable()
	if err != nil {
		return result, fsError("locate running executable", err)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return result, fsError("resolve executable path", err)
	}
	target, err := filepath.EvalSymlinks(source)
	if err != nil {
		return result, fsError("resolve executable symlinks", err)
	}
	if packageManaged(source) || packageManaged(target) {
		return result, fail("INSTALL_METHOD", "package-managed installation at %s; upgrade with its package manager or manually install a standalone binary", target)
	}
	result, err = c.Check(ctx, current)
	if err != nil {
		return result, err
	}
	result.Destination = target
	if !*result.Available {
		return result, nil
	}
	dir := filepath.Dir(target)
	lock, err := acquire(filepath.Join(dir, "."+filepath.Base(target)+".upgrade.lock"))
	if err != nil {
		return result, err
	}
	defer func() {
		if e := lock.Close(); e != nil && err == nil {
			err = fsError("close upgrade lock", e)
		}
	}()
	before, err := inspect(target)
	if err != nil {
		return result, fsError("inspect installation", err)
	}
	if before.info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || before.info.Mode().Perm()&0111 == 0 {
		return result, fail("INSTALL_METHOD", "installation must be a regular executable without setuid/setgid bits")
	}
	if st, ok := before.info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() {
		return result, fail("PERMISSION", "installation is owned by another user; manually install a user-owned standalone binary")
	}
	// Re-execution catches an old process whose destination was replaced before it locked.
	verify := c.probe
	if verify == nil {
		verify = probe
	}
	installed, err := verify(ctx, target)
	if err != nil {
		return result, err
	}
	if installed != current {
		return result, fail("CONFLICT", "installed binary differs from this running process; rerun the installed binary")
	}
	directory, err := os.Open(dir)
	if err != nil {
		return result, fsError("open installation directory", err)
	}
	defer directory.Close()
	dirInfo, err := directory.Stat()
	if err != nil {
		return result, fsError("inspect installation directory", err)
	}
	stage, err := os.CreateTemp(dir, ".wrk-upgrade-*")
	if err != nil {
		return result, fsError("stage upgrade", err)
	}
	stagePath := stage.Name()
	defer func() {
		stage.Close()
		if stagePath != "" {
			if e := os.Remove(stagePath); e != nil && !errors.Is(e, os.ErrNotExist) {
				if err == nil {
					err = fail("CLEANUP_FAILED", "remove upgrade stage %s: %v", stagePath, e)
				} else {
					err = fmt.Errorf("%w; remove abandoned stage %s: %v", err, stagePath, e)
				}
			}
		}
	}()
	manifest, err := c.get(ctx, result.manifest.URL, maxManifest)
	if err != nil {
		return result, err
	}
	archive, err := c.get(ctx, result.archive.URL, maxArchive)
	if err != nil {
		return result, err
	}
	if err = verifyChecksum(manifest, archive, result.archive.Name); err != nil {
		return result, err
	}
	if err = extract(archive, stage); err != nil {
		return result, err
	}
	if err = stage.Chmod(before.info.Mode().Perm()); err != nil {
		return result, fsError("set executable permissions", err)
	}
	if err = stage.Sync(); err != nil {
		return result, fsError("sync executable", err)
	}
	stageInfo, err := stage.Stat()
	if err != nil {
		return result, fsError("inspect stage", err)
	}
	if err = stage.Close(); err != nil {
		return result, fsError("close executable", err)
	}
	candidate, err := verify(ctx, stagePath)
	if err != nil {
		return result, err
	}
	if candidate.Version != result.LatestVersion || candidate.GOOS != current.GOOS || candidate.GOARCH != current.GOARCH || !candidate.IsRelease() {
		return result, fail("CANDIDATE_INVALID", "candidate version/platform does not match the selected stable release")
	}
	if c.beforePublish != nil {
		c.beforePublish()
	}
	if err = ctx.Err(); err != nil {
		return result, fail("NETWORK", "upgrade canceled before publication: %v", err)
	}
	after, err := inspect(target)
	resolved, resolveErr := filepath.EvalSymlinks(source)
	nowDir, dirErr := os.Stat(dir)
	nowStage, stageErr := os.Lstat(stagePath)
	if err != nil || resolveErr != nil || dirErr != nil || stageErr != nil || resolved != target || !before.equal(after) || !os.SameFile(dirInfo, nowDir) || !os.SameFile(stageInfo, nowStage) {
		return result, fail("CONFLICT", "installation, symlink, directory, or staged executable changed before replacement; rerun the installed binary")
	}
	rename := c.rename
	if rename == nil {
		rename = os.Rename
	}
	if err = rename(stagePath, target); err != nil {
		return result, fsError("replace executable", err)
	}
	stagePath = ""
	result.Changed, result.Publication = true, "committed"
	syncDir := c.syncDir
	if syncDir == nil {
		syncDir = (*os.File).Sync
	}
	if err = syncDir(directory); err != nil {
		return result, fail("DURABILITY_UNCERTAIN", "executable replaced but directory sync failed: %v; inspect %s version before retrying", err, target)
	}
	return result, nil
}

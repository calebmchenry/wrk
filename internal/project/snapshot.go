package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"wrk/internal/diagnostic"
	"wrk/internal/ticket"
)

type File struct {
	Data []byte
	Info os.FileInfo
}
type Snapshot struct {
	Root        string
	Config      *Config
	Files       map[string]File
	Names       []string
	Tickets     []*ticket.Ticket
	ByID        map[string]*ticket.Ticket
	Diagnostics []diagnostic.Diagnostic
}

func candidateNames(dir string) ([]string, error) {
	return candidateNamesContext(context.Background(), dir, 0)
}

func candidateNamesContext(ctx context.Context, dir string, maxEntries int) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names := []string{}
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := f.ReadDir(256)
		if err != nil && err != io.EOF {
			return nil, err
		}
		count += len(entries)
		if maxEntries > 0 && count > maxEntries {
			return nil, errReadLimit
		}
		for _, e := range entries {
			if ticket.FilenamePattern.MatchString(e.Name()) {
				names = append(names, e.Name())
			}
		}
		if err == io.EOF {
			break
		}
	}
	sort.Strings(names)
	return names, nil
}

func Load(root string) *Snapshot {
	return LoadContext(context.Background(), root, ReadLimits{})
}

// ReadLimits are optional service resource limits, not project-format limits.
// Zero values retain the ordinary CLI's unbounded loading behavior.
type ReadLimits struct {
	FileBytes, TotalBytes int64
	DirectoryEntries      int
}

var errReadLimit = errors.New("project read limit exceeded; reduce project/file size or use the CLI")

// LoadContext uses the same strict validation as Load, with cancellation between
// file reads and validation phases. YAML parsing itself is not preemptible.
func LoadContext(ctx context.Context, root string, limits ReadLimits) *Snapshot {
	s := &Snapshot{Root: root, Files: map[string]File{}, Names: []string{}, Tickets: []*ticket.Ticket{}, ByID: map[string]*ticket.Ticket{}, Diagnostics: []diagnostic.Diagnostic{}}
	abort := func(err error, path string) bool {
		code := ""
		if errors.Is(err, errReadLimit) {
			code = "RESOURCE_LIMIT"
		} else if ctx.Err() != nil {
			code, err = "CANCELED", ctx.Err()
		}
		if code == "" {
			return false
		}
		s.Diagnostics = append(s.Diagnostics, diagnostic.New(code, err.Error(), path))
		return true
	}
	if abort(ctx.Err(), ".wrk") {
		return s
	}
	var total int64
	read := func(path string) ([]byte, os.FileInfo, error) {
		limit := limits.FileBytes
		if limits.TotalBytes > 0 {
			remaining := limits.TotalBytes - total
			if remaining <= 0 {
				return nil, nil, errReadLimit
			}
			if limit == 0 || remaining < limit {
				limit = remaining
			}
		}
		data, info, err := readRegularContext(ctx, filepath.Join(root, path), limit)
		total += int64(len(data))
		return data, info, err
	}
	dir := filepath.Join(root, ".wrk")
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		s.Diagnostics = append(s.Diagnostics, diagnostic.New("INVALID_PROJECT", ".wrk must be an existing real directory", ".wrk"))
		return s
	}
	data, info, err := read(ConfigPath)
	if abort(err, ConfigPath) {
		return s
	}
	configValid := false
	if err != nil {
		s.Diagnostics = append(s.Diagnostics, diagnostic.New("INVALID_CONFIG", "cannot read regular config: "+err.Error(), ConfigPath))
	} else {
		s.Files[ConfigPath] = File{data, info}
		var ds []diagnostic.Diagnostic
		s.Config, ds = ParseConfig(data)
		s.Diagnostics = append(s.Diagnostics, ds...)
		configValid = len(ds) == 0
	}
	s.Names, err = candidateNamesContext(ctx, dir, limits.DirectoryEntries)
	if abort(err, ".wrk") {
		return s
	}
	if err != nil {
		s.Diagnostics = append(s.Diagnostics, diagnostic.New("IO", err.Error(), ".wrk"))
		return s
	}
	var defs map[string]ticket.FieldDefinition
	if configValid {
		defs = s.Config.Fields
	}
	graphAvailable := true
	for _, name := range s.Names {
		path := ".wrk/" + name
		data, info, err := read(path)
		if abort(err, path) {
			return s
		}
		if err != nil {
			s.Diagnostics = append(s.Diagnostics, diagnostic.New("INVALID_TICKET", "cannot read regular ticket: "+err.Error(), path))
			graphAvailable = false
			continue
		}
		s.Files[path] = File{data, info}
		t, parseDS := ticket.Parse(data, path)
		s.Tickets = append(s.Tickets, t)
		ds := append(parseDS, ticket.Validate(t, defs)...)
		s.Diagnostics = append(s.Diagnostics, ds...)
		if t.Node == nil {
			graphAvailable = false
		}
		for _, d := range ds {
			if d.Field == "id" || d.Field == "parent" || d.Field == "depends_on" || d.Field == "" {
				graphAvailable = false
			}
		}
		if t.ID != "" {
			if t.ID != strings.TrimSuffix(name, ".md") {
				s.Diagnostics = append(s.Diagnostics, diagnostic.Diagnostic{Code: "ID_MISMATCH", Message: "embedded ID must match filename", Path: path, Field: "id", IDs: []string{t.ID}})
			}
			if prior, exists := s.ByID[t.ID]; exists {
				s.Diagnostics = append(s.Diagnostics, diagnostic.Diagnostic{Code: "DUPLICATE_ID", Message: "embedded ID also occurs at " + prior.Path, Path: path, Field: "id", IDs: []string{t.ID}})
				graphAvailable = false
			} else {
				s.ByID[t.ID] = t
			}
		}
	}
	if abort(ctx.Err(), ".wrk") {
		return s
	}
	if !configValid {
		s.Diagnostics = append(s.Diagnostics, diagnostic.New("CHECK_UNAVAILABLE", "configured custom-field checks unavailable until configuration is valid", ConfigPath))
	}
	if graphAvailable {
		s.Diagnostics = append(s.Diagnostics, ValidateGraphs(s)...)
	} else {
		s.Diagnostics = append(s.Diagnostics, diagnostic.New("CHECK_UNAVAILABLE", "relationship checks unavailable until ticket identities and relationship fields are unambiguous", ".wrk"))
	}
	if abort(ctx.Err(), ".wrk") {
		return s
	}
	if len(s.Diagnostics) > 0 {
		s.Diagnostics = append(s.Diagnostics, diagnostic.New("PROJECT_INVALID", "restore a known-good file or obtain explicitly authorized repair; normal commands require a valid project", ".wrk"))
	}
	diagnostic.Sort(s.Diagnostics)
	return s
}

// Compare checks every validation input, including inventory, byte content,
// inode identity, and mode. Timestamp equality is deliberately insufficient.
func (s *Snapshot) Compare() error {
	names, err := candidateNames(filepath.Join(s.Root, ".wrk"))
	if err != nil {
		return fmt.Errorf("CONFLICT: ticket inventory changed: %w", err)
	}
	if len(names) != len(s.Names) {
		return fmt.Errorf("CONFLICT: ticket inventory changed")
	}
	for i := range names {
		if names[i] != s.Names[i] {
			return fmt.Errorf("CONFLICT: ticket inventory changed")
		}
	}
	paths := make([]string, 0, len(s.Files))
	for path := range s.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		original := s.Files[path]
		data, info, err := readRegular(filepath.Join(s.Root, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(data, original.Data) || !os.SameFile(info, original.Info) || info.Mode() != original.Info.Mode() {
			return fmt.Errorf("CONFLICT: %s changed since validation", path)
		}
	}
	return nil
}

func (s *Snapshot) Candidate(path string, data []byte) (*ticket.Ticket, []diagnostic.Diagnostic) {
	t, ds := ticket.Parse(data, path)
	ds = append(ds, ticket.Validate(t, s.Config.Fields)...)
	if len(ds) > 0 {
		return t, ds
	}
	copy := &Snapshot{Root: s.Root, Config: s.Config, ByID: map[string]*ticket.Ticket{}, Tickets: []*ticket.Ticket{}}
	for _, existing := range s.Tickets {
		if existing.Path != path {
			copy.Tickets = append(copy.Tickets, existing)
			copy.ByID[existing.ID] = existing
		}
	}
	if _, exists := copy.ByID[t.ID]; exists {
		ds = append(ds, diagnostic.New("DUPLICATE_ID", "candidate ID is already present", path))
	}
	if filepath.Base(path) != t.ID+".md" {
		ds = append(ds, diagnostic.New("ID_MISMATCH", "candidate ID does not match path", path))
	}
	copy.Tickets = append(copy.Tickets, t)
	copy.ByID[t.ID] = t
	ds = append(ds, ValidateGraphs(copy)...)
	diagnostic.Sort(ds)
	return t, ds
}

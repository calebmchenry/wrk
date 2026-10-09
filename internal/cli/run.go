package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"wrk/internal/buildinfo"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/store"
	"wrk/internal/ticket"
	"wrk/internal/upgrade"
)

func Run(args []string, cwd string, in io.Reader, out, errout io.Writer) int {
	return run(args, cwd, in, out, errout, upgrade.NewClient(), buildinfo.Current())
}

func run(args []string, cwd string, in io.Reader, out, errout io.Writer, updater *upgrade.Client, info buildinfo.Info) int {
	r, err := Parse(args)
	r.JSON = r.JSON || JSONRequested(args)
	e := Envelope{SchemaVersion: 1, Command: r.Command, Errors: []diagnostic.Diagnostic{}}
	if err != nil {
		e.Errors = append(e.Errors, diagnostic.New("USAGE", err.Error(), ""))
		return render(out, errout, r, e, "", 2)
	}
	if r.Help {
		e.OK = true
		e.Result = map[string]any{"usage": Usage}
		return render(out, errout, r, e, Usage, 0)
	}
	if r.Command == "version" {
		e.OK, e.Result = true, info
		return render(out, errout, r, e, fmt.Sprintf("wrk %s (%s, commit %s, %s/%s)", info.Version, info.BuildKind, info.Commit, info.GOOS, info.GOARCH), 0)
	}
	if r.Command == "upgrade" {
		var result upgrade.Result
		if r.Check {
			result, err = updater.Check(context.Background(), info)
		} else {
			result, err = updater.Install(context.Background(), info)
		}
		human, code := "", 0
		e.OK = err == nil
		if e.OK || result.Changed {
			e.Result, human = result, result.Human()
		}
		if err != nil {
			code = 1
			if result.Changed {
				human += "; publication committed; inspect the installed version before retrying"
			}
			name := "IO"
			var failure *upgrade.Error
			if errors.As(err, &failure) {
				name = failure.Code
			}
			e.Errors = append(e.Errors, diagnostic.New(name, err.Error(), ""))
		}
		return render(out, errout, r, e, human, code)
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			e.Errors = append(e.Errors, diagnostic.New("IO", err.Error(), ""))
			return render(out, errout, r, e, "", 1)
		}
	}
	if r.Command == "init" {
		target := cwd
		if len(r.Args) > 0 {
			target = r.Args[0]
			if !filepath.IsAbs(target) {
				target = filepath.Join(cwd, target)
			}
		}
		result := store.Init(target)
		if result.Root != "" {
			e.ProjectRoot = &result.Root
		}
		e.Errors = result.Diagnostics
		e.OK = len(e.Errors) == 0
		human := ""
		code := 0
		if e.OK || result.Committed {
			m := map[string]any{"created": result.Created, "path": ".wrk", "config_path": project.ConfigPath}
			if result.Committed && !e.OK {
				m["publication"] = "committed"
			}
			e.Result = m
			human = "Initialized " + filepath.Join(result.Root, ".wrk")
		}
		if !e.OK {
			code = 1
			if result.Committed {
				human += " (publication committed; inspect before retrying)"
			}
		}
		return render(out, errout, r, e, human, code)
	}
	var body []byte
	if r.BodyFile != nil {
		var err error
		if *r.BodyFile == "-" {
			body, err = io.ReadAll(in)
		} else {
			path := *r.BodyFile
			if !filepath.IsAbs(path) {
				path = filepath.Join(cwd, path)
			}
			body, err = os.ReadFile(path)
		}
		if err != nil {
			e.Errors = append(e.Errors, diagnostic.New("IO", "read body input: "+err.Error(), ""))
			return render(out, errout, r, e, "", 1)
		}
		if !utf8.Valid(body) {
			e.Errors = append(e.Errors, diagnostic.New("INVALID_BODY", "body input must be UTF-8", ""))
			return render(out, errout, r, e, "", 1)
		}
	}
	root, ds := project.Resolve(cwd, r.Selection)
	if root != "" {
		e.ProjectRoot = &root
	}
	e.Errors = ds
	if len(ds) > 0 {
		return render(out, errout, r, e, "", 1)
	}
	if r.Command == "new" {
		result := store.Create(root, store.CreateOptions{Title: r.Args[0], Body: body, Parent: r.Parent, Priority: r.Priority, Labels: r.Labels, LabelsSet: r.NoLabels || len(r.Labels) > 0, Dependencies: r.Dependencies, Fields: r.Fields})
		return renderMutation(out, errout, r, e, result)
	}
	if r.Command == "update" {
		result := store.UpdateWithOptions(root, r.Args[0], store.UpdateOptions{Changes: r.changes(body), Recursive: r.Recursive})
		return renderMutation(out, errout, r, e, result)
	}
	s := project.Load(root)
	e.Errors = s.Diagnostics
	if len(e.Errors) > 0 {
		return render(out, errout, r, e, "", 1)
	}
	human := ""
	switch r.Command {
	case "validate":
		e.Result = map[string]any{"valid": true, "ticket_count": len(s.Tickets)}
		human = fmt.Sprintf("Valid project: %d tickets", len(s.Tickets))
	case "list":
		if r.Under != nil && (!ticket.IDPattern.MatchString(*r.Under) || s.ByID[*r.Under] == nil) {
			e.Errors = append(e.Errors, diagnostic.New("NOT_FOUND", "scope root not found in this project", ""))
			break
		}
		tickets := s.ScopedList(r.All, r.Ready, r.Labels, r.Under)
		e.Result = map[string]any{"tickets": tickets}
		human = listText(tickets)
	case "show":
		if !ticket.IDPattern.MatchString(r.Args[0]) || s.ByID[r.Args[0]] == nil {
			e.Errors = append(e.Errors, diagnostic.New("NOT_FOUND", "ticket not found in this project", ""))
			break
		}
		t := s.ByID[r.Args[0]]
		summary := s.Summary(t)
		children := s.Children(t.ID)
		e.Result = map[string]any{"ticket": summary, "source": string(t.Source), "children": children}
		human = string(t.Source) + "\n--- Derived information ---\nChildren:\n" + listText(children) + "\nBlockers: " + blockerText(summary.Blockers)
	}
	e.OK = len(e.Errors) == 0
	code := 0
	if !e.OK {
		code = 1
	}
	return render(out, errout, r, e, human, code)
}
func blockerText(blockers []project.Blocker) string {
	ss := []string{}
	for _, b := range blockers {
		ss = append(ss, b.ID+" ("+b.Status+")")
	}
	if len(ss) == 0 {
		return "none"
	}
	return strings.Join(ss, ", ")
}
func listText(tickets []project.Summary) string {
	lines := []string{"ID\tSTATUS\tPRIORITY\tTITLE\tBLOCKERS"}
	for _, t := range tickets {
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s\t%s", t.ID, t.Status, t.Priority, t.Title, blockerText(t.Blockers)))
	}
	return strings.Join(lines, "\n")
}

func renderMutation(out, errout io.Writer, r Request, e Envelope, m store.Mutation) int {
	if m.Updates != nil {
		return renderRecursiveMutation(out, errout, r, e, m)
	}
	e.Errors = m.Diagnostics
	e.OK = len(e.Errors) == 0
	human := ""
	if e.OK || m.Committed {
		summary := m.Snapshot.Summary(m.Ticket)
		result := map[string]any{"ticket": summary}
		if r.Command == "new" {
			result["created"] = m.Created
		} else {
			result["changed"] = m.Changed
		}
		if m.Committed && !e.OK {
			result["publication"] = "committed"
		}
		e.Result = result
		human = fmt.Sprintf("%s %s", summary.ID, summary.Path)
		if !m.Changed && !m.Created {
			human += " (unchanged)"
		}
		if m.Committed && !e.OK {
			human += " (publication committed; inspect before retrying)"
		}
	}
	code := 0
	if !e.OK {
		code = 1
	}
	return render(out, errout, r, e, human, code)
}

func renderRecursiveMutation(out, errout io.Writer, r Request, e Envelope, m store.Mutation) int {
	e.Errors = m.Diagnostics
	e.OK = len(e.Errors) == 0
	human := ""
	if e.OK || m.Committed {
		type entry struct {
			Ticket      project.Summary `json:"ticket"`
			Changed     bool            `json:"changed"`
			Publication string          `json:"publication"`
		}
		entries := make([]entry, 0, len(m.Updates))
		lines := []string{"ID\tPATH\tPUBLICATION"}
		for _, update := range m.Updates {
			summary := m.Snapshot.Summary(update.Ticket)
			entries = append(entries, entry{summary, update.Publication == "committed", update.Publication})
			lines = append(lines, fmt.Sprintf("%s\t%s\t%s", summary.ID, summary.Path, update.Publication))
		}
		result := map[string]any{"root_id": m.Ticket.ID, "updates": entries, "changed": m.Changed}
		if m.Committed && !e.OK {
			result["publication"] = "committed"
			lines = append(lines, "Partial or uncertain publication; inspect all targets before retrying.")
		}
		e.Result = result
		human = strings.Join(lines, "\n")
	}
	code := 0
	if !e.OK {
		code = 1
	}
	return render(out, errout, r, e, human, code)
}

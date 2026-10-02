package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"wrk/internal/diagnostic"
	"wrk/internal/project"
	"wrk/internal/store"
	"wrk/internal/ticket"
)

func Run(args []string, cwd string, in io.Reader, out, errout io.Writer) int {
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
	if r.Command == "new" && r.BodyFile != nil {
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
	root, ds := project.Discover(cwd)
	if root != "" {
		e.ProjectRoot = &root
	}
	e.Errors = ds
	if len(ds) > 0 {
		return render(out, errout, r, e, "", 1)
	}
	if r.Command == "new" {
		result := store.Create(root, store.CreateOptions{Title: r.Args[0], Body: body, Parent: r.Parent, Priority: r.Priority, Labels: r.Labels, LabelsSet: r.NoLabels || len(r.Labels) > 0})
		return renderMutation(out, errout, r, e, result)
	}
	if r.Command == "update" {
		result := store.Update(root, r.Args[0], r.Title, r.Status)
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
		tickets := s.List(r.All, r.Ready)
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

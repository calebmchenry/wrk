package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"
	"wrk/internal/ticket"
)

type Request struct {
	Command                                          string
	JSON, Help, All, Ready, NoLabels, Recursive      bool
	Args                                             []string
	Title, Status, BodyFile, Parent, Priority, Under *string
	Labels, AddLabels, RemoveLabels                  []string
}

var commandFlags = map[string]map[string]bool{
	"init": {}, "new": {"body-file": true, "parent": true, "priority": true, "label": true, "no-labels": false},
	"list": {"all": false, "ready": false, "label": true, "under": true}, "show": {}, "update": {"title": true, "status": true, "priority": true, "label": true, "no-labels": false, "add-label": true, "remove-label": true, "recursive": false}, "validate": {}, "help": {},
}

func Parse(args []string) (Request, error) {
	r := Request{Labels: []string{}}
	seen := map[string]bool{}
	options := true
	for i := 0; i < len(args); i++ {
		a := args[i]
		if options && a == "--" {
			options = false
			continue
		}
		if options && strings.HasPrefix(a, "-") && a != "-" {
			name, value, has := strings.Cut(strings.TrimPrefix(a, "--"), "=")
			if a == "-h" {
				name = "help"
			}
			takesValue := false
			if name != "json" && name != "help" {
				var ok bool
				takesValue, ok = commandFlags[r.Command][name]
				if !ok {
					return r, fmt.Errorf("unknown flag %s for %s", a, r.Command)
				}
			}
			if name != "label" && name != "add-label" && name != "remove-label" && seen[name] {
				return r, fmt.Errorf("flag --%s may only be supplied once", name)
			}
			seen[name] = true
			if takesValue {
				if !has {
					if i+1 == len(args) || strings.HasPrefix(args[i+1], "--") {
						return r, fmt.Errorf("--%s requires a value", name)
					}
					i++
					value = args[i]
				}
			} else if has {
				return r, fmt.Errorf("--%s does not take a value", name)
			}
			switch name {
			case "json":
				r.JSON = true
			case "help":
				r.Help = true
			case "all":
				r.All = true
			case "ready":
				r.Ready = true
			case "no-labels":
				r.NoLabels = true
			case "recursive":
				r.Recursive = true
			case "under":
				r.Under = &value
			case "add-label":
				r.AddLabels = append(r.AddLabels, value)
			case "remove-label":
				r.RemoveLabels = append(r.RemoveLabels, value)
			case "label":
				r.Labels = append(r.Labels, value)
			case "title":
				r.Title = &value
			case "status":
				r.Status = &value
			case "body-file":
				r.BodyFile = &value
			case "parent":
				r.Parent = &value
			case "priority":
				r.Priority = &value
			}
		} else if r.Command == "" {
			if _, ok := commandFlags[a]; !ok {
				return r, fmt.Errorf("unknown command %q", a)
			}
			r.Command = a
		} else {
			r.Args = append(r.Args, a)
		}
	}
	if r.Command == "" {
		r.Command = "help"
		r.Help = true
	}
	if r.Command == "help" {
		r.Help = true
		if len(r.Args) > 1 {
			return r, fmt.Errorf("help accepts at most one command")
		}
		if len(r.Args) == 1 {
			if _, ok := commandFlags[r.Args[0]]; !ok {
				return r, fmt.Errorf("unknown command %q", r.Args[0])
			}
		}
	}
	if r.Help {
		return r, nil
	}
	if r.All && r.Ready {
		return r, fmt.Errorf("--all and --ready conflict")
	}
	if r.NoLabels && len(r.Labels) > 0 {
		return r, fmt.Errorf("--no-labels and --label conflict")
	}
	min, max := 0, 0
	switch r.Command {
	case "init":
		max = 1
	case "new", "show", "update":
		min, max = 1, 1
	}
	if len(r.Args) < min || len(r.Args) > max {
		return r, fmt.Errorf("%s expects %d to %d positional arguments", r.Command, min, max)
	}
	if r.Command == "update" {
		if err := r.changes().Validate(); err != nil {
			return r, err
		}
		if r.Recursive && (r.Title != nil || r.Status != nil || r.Priority != nil) {
			return r, fmt.Errorf("--recursive permits only label changes")
		}
	}
	if r.Command == "list" {
		for _, label := range r.Labels {
			if label == "" || !utf8.ValidString(label) {
				return r, fmt.Errorf("--label requires a nonempty UTF-8 string")
			}
		}
	}
	return r, nil
}

// changes translates the command's label mode once for validation and mutation.
func (r Request) changes() ticket.Changes {
	return ticket.Changes{
		Title: r.Title, Status: r.Status, Priority: r.Priority,
		AddLabels: r.AddLabels, RemoveLabels: r.RemoveLabels,
		Labels: r.Labels, LabelsSet: r.NoLabels || len(r.Labels) > 0,
	}
}

// JSONRequested also recognizes JSON on a malformed invocation, respecting --.
func JSONRequested(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "--json" {
			return true
		}
	}
	return false
}

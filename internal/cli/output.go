package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"wrk/internal/diagnostic"
)

const Usage = `wrk — local, Git-tracked project tickets

Usage:
  wrk version | --version
  wrk upgrade [--check]
  wrk init [directory]
  wrk new "Title" [--body-file path|-] [--parent id] [--priority value]
                  [--label value ... | --no-labels] [--depends-on id ...]
                  [--field name=YAML ...]
  wrk list [--all | --ready] [--label value ...] [--under id]
  wrk show <id>
  wrk update <id> [--title "Title"] [--status todo|in-progress|blocked|done|canceled]
                  [--body-file path|-]
                  [--priority low|normal|high|urgent]
                  [--label value ... | --no-labels |
                   --add-label value ... --remove-label value ...] [--recursive]
                  [--parent id | --no-parent]
                  [--add-dependency id ...] [--remove-dependency id ...]
                  [--field name=YAML ...] [--remove-field name ...]
  wrk validate
  wrk help [command]

Project selection for new, list, show, update, and validate:
  --project directory | --config directory/.wrk/config.yaml
Selectors may precede or follow the command and override cwd discovery.
Relative paths (including --body-file) resolve from the invocation directory.
Explicit selection requires an existing project; it never searches ancestors.
Without a selector, use the nearest .wrk boundary from cwd.
Init uses its positional directory; help/version/upgrade do not use selectors.

Update --label replaces the entire label list; --no-labels clears it.
Update --body-file replaces the exact UTF-8 body; empty input clears it.
Omitting --body-file preserves the body. Body edits can combine with metadata.
Replacement/clear conflict with add/remove; either add or remove may be used alone.
Label filters require ALL labels; --under excludes the root. Filters intersect.
Ready means todo with every dependency done. Blocked stays active, never ready.
Recursive updates include the root and all descendants, regardless of status;
only label changes are allowed. Add/remove of the same label conflicts.
Recursive writes publish one ticket at a time; inspect partial failures before retrying.
Parent/dependency edits validate references and each graph's cycles independently.
Dependency add/remove is idempotent; adding and removing the same ID conflicts.
--field sets a typed YAML value; quote YAML strings that resemble numbers/booleans.
Field names split at the first unescaped =; escape name characters as \= or \\.
Each field value is an independent YAML document.
Duplicate field assignments and setting/removing the same field conflict.
Removing an absent field is a no-op; use explicit null to store a YAML null value.

Version and upgrade work without a project. Upgrade --check writes no files.
Upgrade installs the latest stable release for standalone release binaries only.

Every command supports --json and --help. Flags may precede or follow
positional arguments. Use --flag=value or -- to end option parsing.
`

type Envelope struct {
	SchemaVersion int                     `json:"schema_version"`
	Command       string                  `json:"command"`
	OK            bool                    `json:"ok"`
	ProjectRoot   *string                 `json:"project_root"`
	Result        any                     `json:"result"`
	Errors        []diagnostic.Diagnostic `json:"errors"`
}

func Safe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func render(w, errw io.Writer, r Request, e Envelope, human string, code int) int {
	if r.JSON {
		if err := json.NewEncoder(w).Encode(e); err != nil {
			return 1
		}
		return code
	}
	if human != "" {
		if _, err := fmt.Fprintln(w, Safe(human)); err != nil {
			return 1
		}
	}
	for _, d := range e.Errors {
		location := d.Path
		if d.Field != "" {
			location += " [" + d.Field + "]"
		}
		if d.Line > 0 {
			location += fmt.Sprintf(":%d:%d", d.Line, d.Column)
		}
		if _, err := fmt.Fprintf(errw, "%s: %s %s\n", d.Code, Safe(location), Safe(d.Message)); err != nil {
			return 1
		}
	}
	return code
}

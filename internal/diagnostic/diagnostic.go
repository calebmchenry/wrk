// Package diagnostic defines stable, side-effect-free project diagnostics.
package diagnostic

import "sort"

type Diagnostic struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Path    string   `json:"path,omitempty"`
	Field   string   `json:"field,omitempty"`
	Line    int      `json:"line,omitempty"`
	Column  int      `json:"column,omitempty"`
	IDs     []string `json:"ids,omitempty"`
}

func (d Diagnostic) Error() string { return d.Message }
func New(code, message, path string) Diagnostic {
	return Diagnostic{Code: code, Message: message, Path: path}
}
func Sort(ds []Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
}

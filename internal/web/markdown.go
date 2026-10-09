package web

import (
	"bytes"
	"html"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"wrk/internal/ticket"
)

// Goldmark leaves raw HTML disabled. Override every resource/link node so even
// safe Markdown images cannot trigger requests. No user attributes are enabled.
var markdown = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList),
	goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(safeLinks{}, 100))),
)

func renderMarkdown(body []byte) (string, error) {
	var out bytes.Buffer
	err := markdown.Convert(body, &out)
	return out.String(), err
}

type safeLinks struct{}

func (safeLinks) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindLink, renderLink)
	r.Register(ast.KindAutoLink, renderAutoLink)
	r.Register(ast.KindImage, renderImage)
}

// Check the normalized destination that will actually be emitted. Only explicit
// external web/mail links and ticket filenames are navigable; arbitrary local
// files, protocol-relative URLs and custom/executable schemes remain plain text.
func safeDestination(raw []byte) (href, id string) {
	dest := string(util.URLEscape(raw, true))
	u, err := url.Parse(dest)
	if err != nil {
		return "", ""
	}
	if (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" || u.Scheme == "mailto" {
		return dest, ""
	}
	if u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", ""
	}
	id = strings.TrimSuffix(strings.TrimPrefix(u.Path, "./"), ".md")
	if ticket.IDPattern.MatchString(id) {
		return "#item=" + id, id
	}
	return "", ""
}

func openLink(w util.BufWriter, raw []byte) bool {
	href, id := safeDestination(raw)
	if href == "" {
		return false
	}
	_, _ = w.WriteString(`<a href="` + html.EscapeString(href) + `"`)
	if id != "" {
		_, _ = w.WriteString(` data-item="` + html.EscapeString(id) + `"`)
	} else {
		_, _ = w.WriteString(` target="_blank" rel="noopener noreferrer"`)
	}
	_, _ = w.WriteString(">")
	return true
}

func renderLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		openLink(w, n.Destination)
	} else if href, _ := safeDestination(n.Destination); href != "" {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ast.AutoLink)
		dest := n.URL(source)
		if n.AutoLinkType == ast.AutoLinkEmail {
			dest = append([]byte("mailto:"), dest...)
		}
		linked := openLink(w, dest)
		_, _ = w.WriteString(html.EscapeString(string(n.Label(source))))
		if linked {
			_, _ = w.WriteString("</a>")
		}
	}
	return ast.WalkSkipChildren, nil
}

func renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<span class="image-placeholder">[Image: ` + html.EscapeString(string(node.Text(source))) + `]</span>`)
	}
	return ast.WalkSkipChildren, nil
}

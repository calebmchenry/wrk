package web

import (
	"strings"
	"testing"
)

func TestMarkdownRenderingAndResourcePolicy(t *testing.T) {
	t.Parallel()
	body := []byte("# Heading\n\n**strong** and *emphasis* and `code` and ~~old~~\n\n- [x] Finished\n- [ ] Pending\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n> Quote\n\n```js\n<script>alert(1)</script>\n```\n\n[child](task-00000001.md) [external](https://example.com/?a=1&b=2) <user@example.com>\n\n![remote](https://example.com/track.png) ![local](/api/project)\n\n<script>alert(1)</script>\n<iframe src=\"https://example.com\"></iframe>\n<img src=x onerror=alert(1)>\n")
	got, err := renderMarkdown(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<h1>Heading</h1>", "<strong>strong</strong>", "<em>emphasis</em>", "<code>code</code>", "<del>old</del>", "<table>", "<blockquote>", "disabled", "&lt;script&gt;", `href="#item=task-00000001" data-item="task-00000001"`, `href="https://example.com/?a=1&amp;b=2"`, `rel="noopener noreferrer"`, "mailto:user@example.com", "[Image: remote]", "[Image: local]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
	for _, bad := range []string{"<script", "<iframe", "<img", "onerror", "track.png", `src=`, `href="/api/`} {
		if strings.Contains(got, bad) {
			t.Errorf("unsafe %q: %s", bad, got)
		}
	}
}

func TestMarkdownDangerousAndLocalLinks(t *testing.T) {
	t.Parallel()
	for _, dest := range []string{
		"javascript:alert%281%29", "JaVaScRiPt:alert%281%29", "javascript&colon;alert%281%29",
		"jav&#x61;script:alert%281%29", "java&#x09;script:alert%281%29", "data:text/html,evil",
		"vbscript:evil", "file:///etc/passwd", "//example.com", "/api/project", "../docs/index.md",
		"https&#x3a;//example.com/\"onclick=evil", "#item=task-00000001", "task-00000001.md?bad=x",
	} {
		t.Run(dest, func(t *testing.T) {
			got, err := renderMarkdown([]byte("[label](<" + dest + ">)"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(got, "<a ") && !strings.Contains(got, `href="https://example.com/%22onclick=evil"`) {
				t.Fatalf("unexpected navigable destination: %s", got)
			}
		})
	}
	for _, dest := range []string{"task-00000001", "task-00000001.md", "./task-00000001.md"} {
		got, err := renderMarkdown([]byte("[label](" + dest + ")"))
		if err != nil || !strings.Contains(got, `data-item="task-00000001"`) {
			t.Fatalf("ticket link %s: %s %v", dest, got, err)
		}
	}
}

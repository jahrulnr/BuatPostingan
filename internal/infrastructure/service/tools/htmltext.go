package tools

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	wsCollapse = regexp.MustCompile(`[ \t\r\f\v]+`)
	blankLines = regexp.MustCompile(`\n{3,}`)
)

func extractReadable(body []byte, contentType string) (title, text string) {
	media := contentType
	if i := strings.Index(contentType, ";"); i >= 0 {
		media = strings.TrimSpace(contentType[:i])
	}
	media = strings.ToLower(media)

	if media == "text/plain" || media == "text/markdown" || media == "text/x-markdown" ||
		(strings.HasPrefix(media, "text/") && media != "text/html" && media != "text/xml") {
		text = normalizeText(string(body))
		return "", text
	}

	// Default: treat as HTML (including empty/unknown after allowlist).
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", normalizeText(string(body))
	}
	title = findTitle(doc)
	var b strings.Builder
	walkText(doc, &b)
	return title, normalizeText(b.String())
}

func findTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		return normalizeText(nodeText(n))
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findTitle(c); t != "" {
			return t
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func walkText(n *html.Node, b *strings.Builder) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "noscript", "template", "svg", "iframe":
			return
		case "br", "hr":
			b.WriteByte('\n')
			return
		case "p", "div", "section", "article", "header", "footer", "main",
			"li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre":
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
		}
	}
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkText(c, b)
	}
	if n.Type == html.ElementNode {
		switch n.Data {
		case "p", "div", "section", "article", "li", "tr",
			"h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre":
			b.WriteByte('\n')
		}
	}
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = wsCollapse.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")
	s = blankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

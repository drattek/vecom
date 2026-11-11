package sync

import (
	"html"
	"regexp"
	"strings"
)

var (
	odooHTMLTagPattern  = regexp.MustCompile(`(?i)</?(p|br|div|ul|ol|li|h[1-6]|strong|b|em|span|table)\b[^>]*>`)
	odooBlankLinesSplit = regexp.MustCompile(`\n[ \t]*\n+`)
)

// odooDescriptionHTML converts a plain-text product description into the HTML
// Odoo's description_ecommerce (an Html field) needs: HTML collapses newlines
// and runs of spaces, so the raw text would render as a single line. Blank
// lines split paragraphs (<p>), single newlines become <br>, and runs of lines
// starting with "-", "•" or "*" become a <ul>. Text is HTML-escaped. A
// description that already contains HTML tags is sent untouched.
func odooDescriptionHTML(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	if text == "" || odooHTMLTagPattern.MatchString(text) {
		return text
	}

	var out strings.Builder
	var items []string
	flushList := func() {
		if len(items) == 0 {
			return
		}
		out.WriteString("<ul>")
		for _, item := range items {
			out.WriteString("<li>" + item + "</li>")
		}
		out.WriteString("</ul>")
		items = nil
	}

	for _, block := range odooBlankLinesSplit.Split(text, -1) {
		var lines []string
		for _, line := range strings.Split(block, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			continue
		}

		allBullets := true
		for _, line := range lines {
			if !isOdooBullet(line) {
				allBullets = false
				break
			}
		}
		if allBullets {
			for _, line := range lines {
				items = append(items, html.EscapeString(strings.TrimSpace(line[1:])))
			}
			continue
		}

		flushList()
		escaped := make([]string, len(lines))
		for i, line := range lines {
			escaped[i] = html.EscapeString(line)
		}
		out.WriteString("<p>" + strings.Join(escaped, "<br>") + "</p>")
	}
	flushList()

	return out.String()
}

func isOdooBullet(line string) bool {
	if len(line) < 2 {
		return false
	}
	switch line[0] {
	case '-', '*':
		return strings.TrimSpace(line[1:]) != ""
	}
	return strings.HasPrefix(line, "•") && strings.TrimSpace(strings.TrimPrefix(line, "•")) != ""
}

// Package markdown — крошечное подмножество Markdown для комментариев: жирный, курсив, зачёркнутый,
// код, списки, цитаты, ссылки. Сначала экранируется весь HTML, потом добавляется разметка,
// поэтому в вывод не попадает ничего, кроме наших тегов; ссылки — только http(s), с nofollow.
package markdown

import (
	"html"
	"regexp"
	"strings"
)

var (
	reBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic = regexp.MustCompile(`(^|[^*\w])\*([^*\n]+?)\*`)
	reUnder  = regexp.MustCompile(`(^|[^_\w])_([^_\n]+?)_`)
	reStrike = regexp.MustCompile(`~~(.+?)~~`)
	reCode   = regexp.MustCompile("`([^`\n]+?)`")
	reLink   = regexp.MustCompile(`\[([^\]\n]+?)\]\((https?://[^\s)]+)\)`)
	reURL    = regexp.MustCompile(`(^|[\s(])(https?://[^\s<)]+)`)
)

// Render — HTML для body; пустой ввод → пустая строка.
func Render(src string) string {
	src = strings.ReplaceAll(strings.TrimSpace(src), "\r\n", "\n")
	if src == "" {
		return ""
	}
	var out strings.Builder
	lines := strings.Split(src, "\n")
	list := "" // ul | ol | quote
	closeList := func() {
		switch list {
		case "ul":
			out.WriteString("</ul>")
		case "ol":
			out.WriteString("</ol>")
		case "quote":
			out.WriteString("</blockquote>")
		}
		list = ""
	}
	para := []string{}
	flushPara := func() {
		if len(para) > 0 {
			out.WriteString("<p>" + strings.Join(para, "<br>") + "</p>")
			para = para[:0]
		}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			flushPara()
			closeList()
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• "):
			flushPara()
			if list != "ul" {
				closeList()
				out.WriteString("<ul>")
				list = "ul"
			}
			out.WriteString("<li>" + inline(line[2:]) + "</li>")
		case isOrdered(line):
			flushPara()
			if list != "ol" {
				closeList()
				out.WriteString("<ol>")
				list = "ol"
			}
			out.WriteString("<li>" + inline(line[strings.Index(line, " ")+1:]) + "</li>")
		case strings.HasPrefix(line, "> "):
			flushPara()
			if list != "quote" {
				closeList()
				out.WriteString("<blockquote>")
				list = "quote"
			} else {
				out.WriteString("<br>")
			}
			out.WriteString(inline(line[2:]))
		default:
			closeList()
			para = append(para, inline(line))
		}
	}
	flushPara()
	closeList()
	return out.String()
}

func isOrdered(s string) bool {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i > 0 && i < 4 && i+1 < len(s) && s[i] == '.' && s[i+1] == ' '
}

func inline(s string) string {
	s = html.EscapeString(s)
	s = reCode.ReplaceAllString(s, "<code>$1</code>")
	s = reLink.ReplaceAllString(s, `<a href="$2" rel="nofollow noopener" target="_blank">$1</a>`)
	s = reURL.ReplaceAllString(s, `$1<a href="$2" rel="nofollow noopener" target="_blank">$2</a>`)
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reItalic.ReplaceAllString(s, "$1<em>$2</em>")
	s = reUnder.ReplaceAllString(s, "$1<em>$2</em>")
	s = reStrike.ReplaceAllString(s, "<s>$1</s>")
	return s
}

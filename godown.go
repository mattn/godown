package godown

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"

	"golang.org/x/net/html"
)

func isChildOf(node *html.Node, name string) bool {
	node = node.Parent
	return node != nil && node.Type == html.ElementNode && strings.ToLower(node.Data) == name
}

func hasClass(node *html.Node, clazz string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == clazz {
					return true
				}
			}
		}
	}
	return false
}

func attr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// Gets the language of a code block based on the class
// See: https://spec.commonmark.org/0.29/#example-112
func langFromClass(node *html.Node) string {
	if node.FirstChild == nil || strings.ToLower(node.FirstChild.Data) != "code" {
		return ""
	}

	fChild := node.FirstChild
	classes := strings.Fields(attr(fChild, "class"))
	if len(classes) == 0 {
		return ""
	}

	prefix := "language-"
	for _, class := range classes {
		if !strings.HasPrefix(class, prefix) {
			continue
		}
		return strings.TrimPrefix(class, prefix)
	}

	return ""
}

func br(node *html.Node, w io.Writer, option *Option) {
	node = node.PrevSibling
	if node == nil {
		return
	}
	switch node.Type {
	case html.TextNode:
		text := strings.Trim(node.Data, " \t")
		if text != "" && !strings.HasSuffix(text, "\n") {
			fmt.Fprint(w, "\n")
		}
	case html.ElementNode:
		switch strings.ToLower(node.Data) {
		case "br", "p", "ul", "ol", "div", "blockquote", "h1", "h2", "h3", "h4", "h5", "h6":
			fmt.Fprint(w, "\n")
		}
	}
}

func table(node *html.Node, w io.Writer, option *Option) {
	for tr := node.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type == html.ElementNode && strings.ToLower(tr.Data) == "tbody" {
			node = tr
			break
		}
	}
	var header bool
	var rows [][]string
	for tr := node.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode || strings.ToLower(tr.Data) != "tr" {
			continue
		}
		var cols []string
		if !header {
			for th := tr.FirstChild; th != nil; th = th.NextSibling {
				if th.Type != html.ElementNode || strings.ToLower(th.Data) != "th" {
					continue
				}
				var buf bytes.Buffer
				walk(th, &buf, 0, option)
				cols = append(cols, buf.String())
			}
			if len(cols) > 0 {
				rows = append(rows, cols)
				header = true
				continue
			}
		}
		for td := tr.FirstChild; td != nil; td = td.NextSibling {
			if td.Type != html.ElementNode || strings.ToLower(td.Data) != "td" {
				continue
			}
			var buf bytes.Buffer
			walk(td, &buf, 0, option)
			cols = append(cols, buf.String())
		}
		rows = append(rows, cols)
	}
	maxcol := 0
	for _, cols := range rows {
		if len(cols) > maxcol {
			maxcol = len(cols)
		}
	}
	widths := make([]int, maxcol)
	for _, cols := range rows {
		for i := 0; i < maxcol; i++ {
			if i < len(cols) {
				width := runewidth.StringWidth(cols[i])
				if widths[i] < width {
					widths[i] = width
				}
			}
		}
	}
	for i, cols := range rows {
		for j := 0; j < maxcol; j++ {
			fmt.Fprint(w, "|")
			if j < len(cols) {
				width := runewidth.StringWidth(cols[j])
				fmt.Fprint(w, cols[j])
				fmt.Fprint(w, strings.Repeat(" ", widths[j]-width))
			} else {
				fmt.Fprint(w, strings.Repeat(" ", widths[j]))
			}
		}
		fmt.Fprint(w, "|\n")
		if i == 0 && header {
			for j := 0; j < maxcol; j++ {
				fmt.Fprint(w, "|")
				fmt.Fprint(w, strings.Repeat("-", widths[j]))
			}
			fmt.Fprint(w, "|\n")
		}
	}
	fmt.Fprint(w, "\n")
}

var emptyElements = []string{
	"area",
	"base",
	"br",
	"col",
	"embed",
	"hr",
	"img",
	"input",
	"keygen",
	"link",
	"meta",
	"param",
	"source",
	"track",
	"wbr",
}

func raw(node *html.Node, w io.Writer, option *Option) {
	switch node.Type {
	case html.ElementNode:
		fmt.Fprintf(w, "<%s", node.Data)
		for _, attr := range node.Attr {
			fmt.Fprintf(w, " %s=%q", attr.Key, attr.Val)
		}
		found := false
		tag := strings.ToLower(node.Data)
		for _, e := range emptyElements {
			if e == tag {
				found = true
				break
			}
		}
		if found {
			fmt.Fprint(w, "/>")
		} else {
			fmt.Fprint(w, ">")
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				raw(c, w, option)
			}
			fmt.Fprintf(w, "</%s>", node.Data)
		}
	case html.TextNode:
		fmt.Fprint(w, node.Data)
	}
}

func bq(node *html.Node, w io.Writer, option *Option) {
	if node.Type == html.TextNode {
		fmt.Fprint(w, strings.Replace(node.Data, "\u00a0", " ", -1))
	} else {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			bq(c, w, option)
		}
	}
}

func pre(node *html.Node, w io.Writer, option *Option) {
	if node.Type == html.TextNode {
		fmt.Fprint(w, node.Data)
	} else {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			pre(c, w, option)
		}
	}
}

// In the spec, https://spec.commonmark.org/0.29/#delimiter-run
// A  left-flanking delimiter run should not followed by Unicode whitespace
// A  right-flanking delimiter run should not preceded by Unicode whitespace
// This will wrap the delimiter (such as **) around the non-whitespace contents, but preserve the whitespace
func aroundNonWhitespace(node *html.Node, w io.Writer, nest int, option *Option, before, after string) {
	buf := &bytes.Buffer{}
	walk(node, buf, nest, option)
	s := buf.String()

	// If the contents are simply whitespace, return without adding any delimiters
	if strings.TrimSpace(s) == "" {
		return
	}

	start := 0
	for ; start < len(s); start++ {
		c := s[start]
		if !unicode.IsSpace(rune(c)) {
			break
		}
	}

	stop := len(s)
	for ; stop > start; stop-- {
		c := s[stop-1]
		if !unicode.IsSpace(rune(c)) {
			break
		}
	}

	s = s[:start] + before + s[start:stop] + after + s[stop:]

	fmt.Fprint(w, s)
}

func walk(node *html.Node, w io.Writer, nest int, option *Option) {
	if node.Type == html.TextNode {
		if strings.TrimSpace(node.Data) != "" {
			text := regexp.MustCompile(`[[:space:]][[:space:]]*`).ReplaceAllString(strings.Trim(node.Data, "\t\r\n"), " ")
			fmt.Fprint(w, text)
		}
	}
	n := 0
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.CommentNode:
			fmt.Fprint(w, "<!--")
			fmt.Fprint(w, c.Data)
			fmt.Fprint(w, "-->\n")
		case html.ElementNode:
			switch strings.ToLower(c.Data) {
			case "a":
				fmt.Fprint(w, "[")
				walk(c, w, nest, option)
				fmt.Fprint(w, "]("+attr(c, "href")+")")
			case "b", "strong":
				aroundNonWhitespace(c, w, nest, option, "**", "**")
			case "i", "em":
				aroundNonWhitespace(c, w, nest, option, "_", "_")
			case "del":
				aroundNonWhitespace(c, w, nest, option, "~~", "~~")
			case "br":
				br(c, w, option)
				fmt.Fprint(w, "\n\n")
			case "p":
				br(c, w, option)
				walk(c, w, nest, option)
				br(c, w, option)
				fmt.Fprint(w, "\n\n")
			case "code":
				if !isChildOf(c, "pre") {
					fmt.Fprint(w, "`")
					pre(c, w, option)
					fmt.Fprint(w, "`")
				}
			case "pre":
				br(c, w, option)
				var buf bytes.Buffer
				pre(c, &buf, option)
				var lang string = langFromClass(c)
				if option != nil && option.GuessLang != nil {
					if guess, err := option.GuessLang(buf.String()); err == nil {
						lang = guess
					}
				}
				fmt.Fprint(w, "```"+lang+"\n")
				fmt.Fprint(w, buf.String())
				if !strings.HasSuffix(buf.String(), "\n") {
					fmt.Fprint(w, "\n")
				}
				fmt.Fprint(w, "```\n\n")
			case "div":
				br(c, w, option)
				walk(c, w, nest, option)
				fmt.Fprint(w, "\n")
			case "blockquote":
				br(c, w, option)
				var buf bytes.Buffer
				if hasClass(c, "code") {
					bq(c, &buf, option)
					var lang string
					if option != nil && option.GuessLang != nil {
						if guess, err := option.GuessLang(buf.String()); err == nil {
							lang = guess
						}
					}
					fmt.Fprint(w, "```"+lang+"\n")
					fmt.Fprint(w, strings.TrimLeft(buf.String(), "\n"))
					if !strings.HasSuffix(buf.String(), "\n") {
						fmt.Fprint(w, "\n")
					}
					fmt.Fprint(w, "```\n\n")
				} else {
					walk(c, &buf, nest+1, option)

					if lines := strings.Split(strings.TrimSpace(buf.String()), "\n"); len(lines) > 0 {
						for _, l := range lines {
							fmt.Fprint(w, "> "+strings.TrimSpace(l)+"\n")
						}
						fmt.Fprint(w, "\n")
					}
				}
			case "ul", "ol":
				br(c, w, option)
				var buf bytes.Buffer
				walk(c, &buf, 1, option)
				if lines := strings.Split(strings.TrimSpace(buf.String()), "\n"); len(lines) > 0 {
					for i, l := range lines {
						if i > 0 {
							fmt.Fprint(w, "\n")
						}
						fmt.Fprint(w, strings.Repeat("    ", nest)+l)
					}
					fmt.Fprint(w, "\n")
				}
			case "li":
				br(c, w, option)
				if isChildOf(c, "ul") {
					fmt.Fprint(w, "* ")
				} else if isChildOf(c, "ol") {
					n++
					fmt.Fprint(w, fmt.Sprintf("%d. ", n))
				}
				walk(c, w, nest, option)
				fmt.Fprint(w, "\n")
			case "h1", "h2", "h3", "h4", "h5", "h6":
				br(c, w, option)
				fmt.Fprint(w, strings.Repeat("#", int(rune(c.Data[1])-rune('0')))+" ")
				walk(c, w, nest, option)
				fmt.Fprint(w, "\n\n")
			case "img":
				src := attr(c, "src")
				alt := attr(c, "alt")
				if src != "" {
					fmt.Fprintf(w, "![%s](%s)", alt, src)
				}
			case "hr":
				br(c, w, option)
				fmt.Fprint(w, "\n---\n\n")
			case "table":
				br(c, w, option)
				table(c, w, option)
			case "style":
				if option != nil && option.Style {
					br(c, w, option)
					raw(c, w, option)
					fmt.Fprint(w, "\n\n")
				}
			case "script":
				if option != nil && option.Script {
					br(c, w, option)
					raw(c, w, option)
					fmt.Fprint(w, "\n\n")
				}
			default:
				if option == nil || option.CustomRules == nil {
					walk(c, w, nest, option)
					break
				}

				foundCustom := false
				for _, cr := range option.CustomRules {
					if tag, customWalk := cr.Rule(walk); strings.ToLower(c.Data) == tag {
						customWalk(c, w, nest, option)
						foundCustom = true
					}
				}

				if foundCustom {
					break
				}
				walk(c, w, nest, option)
			}
		default:
			walk(c, w, nest, option)
		}
	}
}

// WalkFunc type is an signature for functions traversing HTML nodes
type WalkFunc func(node *html.Node, w io.Writer, nest int, option *Option)

// CustomRule is an interface to define custom conversion rules
//
// Rule method accepts `next WalkFunc` as an argument, which `customRule` should call
// to let walk function continue parsing the content inside the HTML tag.
// It returns a tagName to indicate what HTML element this `customRule` handles and the `customRule`
// function itself, where conversion logic should reside.
//
// See example TestRule implementation in godown_test.go
type CustomRule interface {
	Rule(next WalkFunc) (tagName string, customRule WalkFunc)
}

// Option is optional information for Convert.
type Option struct {
	GuessLang   func(string) (string, error)
	Script      bool
	Style       bool
	CustomRules []CustomRule
}

// Convert convert HTML to Markdown. Read HTML from r and write to w.
func Convert(w io.Writer, r io.Reader, option *Option) error {
	doc, err := html.Parse(r)
	if err != nil {
		return err
	}
	walk(doc, w, 0, option)
	fmt.Fprint(w, "\n")
	return nil
}

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

// A regex to escape certain characters
// \ : since this is the excape character, become weird if printed literally
// * : Used to start bullet lists, and as a delimiter
// _ : Used as a delimiter
// ( and ) : Used in links and images
// [ and ] : Used in links and images
// < : can be used to mean "raw HTML" which is allowed
// > : Used in raw HTML, also used to define blockquotes
// # : Used for headings
// + : Can be used for unordered lists
// - : Can be used for unordered lists
// ! : Used for images
// ` : Used for code blocks
var escapeRegex = regexp.MustCompile(`(` + `\\|\*|_|\[|\]|\(|\)|<|>|#|\+|-|!|` + "`" + `)`)

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

	// If trimspace is set to true, new lines will be ignored in nodes
	// so we force a new line when using br()
	if option.TrimSpace {
		fmt.Fprint(w, "\n")
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
	var list []*html.Node // create a list not to mess up the loop

	for tsection := node.FirstChild; tsection != nil; tsection = tsection.NextSibling {
		// if the thead/tbody/tfoot is not explicitly set, it is implicitly set as tbody
		if tsection.Type == html.ElementNode {
			switch strings.ToLower(tsection.Data) {
			case "thead", "tbody", "tfoot":
				for tr := tsection.FirstChild; tr != nil; tr = tr.NextSibling {
					if strings.TrimSpace(tr.Data) == "" {
						continue
					}
					list = append(list, tr)
				}
			}
		}
	}

	// Now we create a new node, add all the <tr> to the node and convert it
	newTableNode := new(html.Node)
	for _, n := range list {
		n.Parent.RemoveChild(n)
		newTableNode.AppendChild(n)
	}

	tableRows(newTableNode, w, option)
	fmt.Fprint(w, "\n")
}

func tableRows(node *html.Node, w io.Writer, option *Option) {
	var rows [][]string
	for tr := node.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode || strings.ToLower(tr.Data) != "tr" {
			continue
		}
		var cols []string
		for td := tr.FirstChild; td != nil; td = td.NextSibling {
			nodeType := strings.ToLower(td.Data)
			if td.Type != html.ElementNode || (nodeType != "td" && nodeType != "th") {
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
		if i == 0 {
			for j := 0; j < maxcol; j++ {
				fmt.Fprint(w, "|")
				fmt.Fprint(w, strings.Repeat("-", widths[j]))
			}
			fmt.Fprint(w, "|\n")
		}
	}
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
	html.Render(w, node)
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
		fmt.Fprint(w, s)
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
		if option.TrimSpace && strings.TrimSpace(node.Data) == "" {
			return
		}

		text := regexp.MustCompile(`[[:space:]][[:space:]]*`).ReplaceAllString(strings.Trim(node.Data, "\t\r\n"), " ")

		if !option.doNotEscape {
			text = escapeRegex.ReplaceAllStringFunc(text, func(str string) string {
				return `\` + str
			})
		}
		fmt.Fprint(w, text)
	}

	n := 0
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.CommentNode:
			fmt.Fprint(w, "<!--")
			fmt.Fprint(w, c.Data)
			fmt.Fprint(w, "-->\n")
		case html.ElementNode:
			customWalk, ok := option.customRulesMap[strings.ToLower(c.Data)]
			if ok {
				customWalk(c, w, nest, option)
				break
			}

			switch strings.ToLower(c.Data) {
			case "a":
				// Links are invalid in markdown if the link text extends beyond a single line
				// So we render the contents and strip any spaces
				href := attr(c, "href")
				end := fmt.Sprintf("](%s)", href)
				title := attr(c, "title")
				if title != "" {
					end = fmt.Sprintf("](%s %q)", href, title)
				}
				aroundNonWhitespace(c, w, nest, option, "[", end)
			case "b", "strong":
				aroundNonWhitespace(c, w, nest, option, "**", "**")
			case "i", "em":
				aroundNonWhitespace(c, w, nest, option, "_", "_")
			case "del", "s":
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

				clone := option.Clone()
				clone.doNotEscape = true

				var buf bytes.Buffer
				pre(c, &buf, clone)
				inner := buf.String()

				var lang string = langFromClass(c)
				if option != nil && option.GuessLang != nil {
					if guess, err := option.GuessLang(buf.String()); err == nil {
						lang = guess
					}
				}

				fmt.Fprint(w, "```"+lang+"\n")
				fmt.Fprint(w, inner)
				if !strings.HasSuffix(inner, "\n") {
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

				var newOption = option.Clone()
				newOption.TrimSpace = true

				var buf bytes.Buffer
				walk(c, &buf, nest+1, newOption)

				// Remove any empty lines in the list
				if lines := strings.Split(buf.String(), "\n"); len(lines) > 0 {
					for i, l := range lines {
						if strings.TrimSpace(l) == "" {
							continue
						}

						if i > 0 {
							fmt.Fprint(w, "\n")
						}

						fmt.Fprint(w, l)
					}
					fmt.Fprint(w, "\n")
					if nest == 0 {
						fmt.Fprint(w, "\n")
					}
				}
			case "li":
				br(c, w, option)

				var buf bytes.Buffer
				walk(c, &buf, 0, option)

				markPrinted := false

				for _, l := range strings.Split(buf.String(), "\n") {
					if strings.TrimSpace(l) == "" {
						continue
					}
					// if markPrinted {

					// }
					if markPrinted {
						fmt.Fprint(w, "\n    ")
					}

					fmt.Fprint(w, strings.Repeat("    ", nest-1))

					if !markPrinted {
						if isChildOf(c, "ul") {
							fmt.Fprint(w, "* ")
						} else if isChildOf(c, "ol") {
							n++
							fmt.Fprint(w, fmt.Sprintf("%d. ", n))
						}

						markPrinted = true
					}

					fmt.Fprint(w, l)
				}

				fmt.Fprint(w, "\n")

			case "h1", "h2", "h3", "h4", "h5", "h6":
				br(c, w, option)
				fmt.Fprint(w, strings.Repeat("#", int(rune(c.Data[1])-rune('0')))+" ")
				walk(c, w, nest, option)
				fmt.Fprint(w, "\n\n")
			case "img":
				src := attr(c, "src")
				alt := attr(c, "alt")
				title := attr(c, "title")

				if src == "" {
					break
				}

				full := fmt.Sprintf("![%s](%s)", alt, src)
				if title != "" {
					full = fmt.Sprintf("![%s](%s %q)", alt, src, title)
				}

				fmt.Fprintf(w, full)
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
	GuessLang      func(string) (string, error)
	Script         bool
	Style          bool
	TrimSpace      bool
	CustomRules    []CustomRule
	doNotEscape    bool // Used to know if to escape certain characters
	customRulesMap map[string]WalkFunc
}

// To make a copy of an option without changing the original
func (o *Option) Clone() *Option {
	if o == nil {
		return nil
	}

	var clone Option
	clone = *o
	return &clone
}

// Convert convert HTML to Markdown. Read HTML from r and write to w.
func Convert(w io.Writer, r io.Reader, option *Option) error {
	doc, err := html.Parse(r)
	if err != nil {
		return err
	}
	if option == nil {
		option = &Option{}
	}

	option.customRulesMap = make(map[string]WalkFunc)
	for _, cr := range option.CustomRules {
		tag, customWalk := cr.Rule(walk)
		option.customRulesMap[tag] = customWalk
	}

	walk(doc, w, 0, option)
	fmt.Fprint(w, "\n")
	return nil
}

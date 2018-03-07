package godown

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

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

func br(node *html.Node, w io.Writer, options *Option) {
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

func table(node *html.Node, w io.Writer, options *Option) {
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
				walk(th, &buf, 0, options)
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
			walk(td, &buf, 0, options)
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

func bq(node *html.Node, w io.Writer, options *Option) {
	if node.Type == html.TextNode {
		fmt.Fprint(w, strings.Replace(node.Data, "\u00a0", " ", -1))
	} else {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			bq(c, w, options)
		}
	}
}

func pre(node *html.Node, w io.Writer, options *Option) {
	if node.Type == html.TextNode {
		fmt.Fprint(w, node.Data)
	} else {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			pre(c, w, options)
		}
	}
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
		case html.ElementNode:
			switch strings.ToLower(c.Data) {
			case "a":
				fmt.Fprint(w, "[")
				walk(c, w, nest, option)
				fmt.Fprint(w, "]("+attr(c, "href")+")")
			case "b", "strong":
				fmt.Fprint(w, "**")
				walk(c, w, nest, option)
				fmt.Fprint(w, "**")
			case "i", "em":
				fmt.Fprint(w, "_")
				walk(c, w, nest, option)
				fmt.Fprint(w, "_")
			case "del":
				fmt.Fprint(w, "~~")
				walk(c, w, nest, option)
				fmt.Fprint(w, "~~")
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
				var lang string
				if option != nil && option.GuessLang != nil {
					lang = option.GuessLang(buf.String())
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
						lang = option.GuessLang(buf.String())
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
						if i > 0 || nest > 0 {
							fmt.Fprint(w, "\n")
						}
						fmt.Fprint(w, strings.Repeat("    ", nest)+strings.TrimSpace(l))
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
				fmt.Fprint(w, "!["+attr(c, "alt")+"]("+attr(c, "src")+")")
			case "hr":
				br(c, w, option)
				fmt.Fprint(w, "\n---\n\n")
			case "table":
				br(c, w, option)
				table(c, w, option)
			default:
				walk(c, w, nest, option)
			}
		default:
			walk(c, w, nest, option)
		}
	}
}

type Option struct {
	GuessLang func(string) string
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

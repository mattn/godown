package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func isChildOf(node *html.Node, name string) bool {
	node = node.Parent
	return node != nil && node.Type == html.ElementNode && strings.ToLower(node.Data) == name
}

func hasClass(node *html.Node, clazz string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Split(attr.Val, " ") {
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

func br(node *html.Node, w io.Writer) {
	node = node.PrevSibling
	if node == nil {
		return
	}
	switch node.Type {
	case html.TextNode:
		text := strings.Trim(node.Data, " ")
		if text != "" && !strings.HasSuffix(text, "\n") {
			fmt.Fprint(w, "\n")
		}
	case html.ElementNode:
		switch strings.ToLower(node.Data) {
		case "p", "ul", "ol", "div", "blockquote":
			fmt.Fprint(w, "\n")
		}
	}
}

func pre(node *html.Node, w io.Writer) {
	if node.Type == html.TextNode {
		fmt.Fprint(w, node.Data)
	} else {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.ElementNode:
				fmt.Fprintf(w, "<%s", c.Data)
				for _, attr := range c.Attr {
					fmt.Fprintf(w, " %s=%q", attr.Key, attr.Val)
				}
				fmt.Fprint(w, ">")
				if c.FirstChild != nil {
					pre(c, w)
					fmt.Fprintf(w, "</%s>", c.Data)
				} else {
					fmt.Fprint(w, "/>")
				}
			default:
				pre(c, w)
			}
		}
	}
}

func walk(node *html.Node, w io.Writer, nest int) {
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
				walk(c, w, nest)
				fmt.Fprint(w, "]("+attr(c, "href")+")")
			case "b", "strong":
				fmt.Fprint(w, "**")
				walk(c, w, nest)
				fmt.Fprint(w, "**")
			case "i", "em":
				fmt.Fprint(w, "_")
				walk(c, w, nest)
				fmt.Fprint(w, "_")
			case "del":
				fmt.Fprint(w, "~~")
				walk(c, w, nest)
				fmt.Fprint(w, "~~")
			case "br":
				fmt.Fprint(w, "\n")
			case "p":
				br(c, w)
				walk(c, w, nest)
				fmt.Fprint(w, "\n")
			case "code":
				fmt.Fprint(w, "`")
				pre(c, w)
				fmt.Fprint(w, "`")
			case "pre":
				br(c, w)
				fmt.Fprint(w, "```\n")
				pre(c, w)
				br(c, w)
				fmt.Fprint(w, "```\n\n")
			case "blockquote":
				br(c, w)
				if hasClass(c, "code") {
					fmt.Fprint(w, "```\n")
					pre(c, w)
					br(c, w)
					fmt.Fprint(w, "```\n")
				} else {
					var buf bytes.Buffer
					walk(c, &buf, nest+1)

					if lines := strings.Split(strings.TrimSpace(buf.String()), "\n"); len(lines) > 0 {
						for _, l := range lines {
							fmt.Fprint(w, "> "+strings.TrimSpace(l)+"\n")
						}
						fmt.Fprint(w, "\n")
					}
				}
			case "ul", "ol":
				walk(c, w, nest+1)
			case "li":
				br(c, w)
				fmt.Fprint(w, strings.Repeat("  ", nest-1))
				if isChildOf(c, "ul") {
					fmt.Fprint(w, "* ")
				} else if isChildOf(c, "ol") {
					n++
					fmt.Fprint(w, fmt.Sprintf("%d. ", n))
				}
				walk(c, w, nest)
				fmt.Fprint(w, "\n")
			case "h1", "h2", "h3", "h4", "h5", "h6":
				br(c, w)
				fmt.Fprint(w, "\n")
				fmt.Fprint(w, strings.Repeat("#", int(rune(c.Data[1])-rune('0')))+" ")
				walk(c, w, nest)
				fmt.Fprint(w, "\n")
			case "img":
				fmt.Fprint(w, "!["+attr(c, "alt")+"]("+attr(c, "src")+")")
			case "hr":
				br(c, w)
				fmt.Fprint(w, "\n---\n")
			default:
				walk(c, w, nest)
			}
		default:
			walk(c, w, nest)
		}
	}
}

func firstBody(node *html.Node) *html.Node {
loop:
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			if c.Data == "body" {
				node = c
				break loop
			}
			if found := firstBody(c); found != c {
				node = found
				break loop
			}
		}
	}
	return node
}

func main() {
	doc, err := html.Parse(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	walk(doc, os.Stdout, 0)
	fmt.Fprint(os.Stdout, "\n")
}

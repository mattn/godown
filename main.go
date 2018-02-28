package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/net/html"
)

var replacer = strings.NewReplacer(
	"\t", " ",
	"\r", "",
	"\n", "",
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
		if node.Data != "" && !strings.HasSuffix(node.Data, "\n") {
			fmt.Fprint(w, "\n")
		}
	case html.ElementNode:
		switch strings.ToLower(node.Data) {
		case "br", "p", "ul", "ol", "blockquote":
			fmt.Fprint(w, "\n")
		}
	}
}

func walk(node *html.Node, w io.Writer, nest int) {
	if node.Type == html.TextNode {
		text := replacer.Replace(strings.TrimLeft(node.Data, " \t\r\n"))
		fmt.Fprint(w, text)
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
				fmt.Fprint(w, "\n")
				walk(c, w, nest)
				fmt.Fprint(w, "\n")
			case "code":
				fmt.Fprint(w, "`")
				walk(c, w, nest)
				fmt.Fprint(w, "`")
			case "blockquote":
				if hasClass(c, "code") {
					br(c, w)
					fmt.Fprint(w, "\n```\n")
					walk(c, w, nest)
					br(c, w)
					fmt.Fprint(w, "```\n")
				} else {
					var buf bytes.Buffer
					walk(c, &buf, nest)

					br(c, w)
					fmt.Fprint(w, "\n")
					for _, l := range strings.Split(buf.String(), "\n") {
						if l != "" {
							fmt.Fprint(w, strings.Repeat("> ", nest+1)+l+"\n")
						}
					}
					fmt.Fprint(w, "\n")
				}
			case "ul", "ol":
				// FIXME: make indentation for the nest level
				walk(c, w, nest+1)
				fmt.Fprint(w, "\n")
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

func main() {
	doc, err := html.Parse(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	walk(doc, os.Stdout, 0)
}

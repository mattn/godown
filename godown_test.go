package godown

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestGodown(t *testing.T) {
	m, err := filepath.Glob("testdata/*.html")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(m)
	for _, file := range m {
		f, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err = Convert(&buf, f, nil); err != nil {
			t.Fatal(err)
		}

		b, err := ioutil.ReadFile(file[:len(file)-4] + "md")
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != buf.String() {
			t.Errorf("(%s):\nwant:\n%s}}}\ngot:\n%s}}}\n", file, string(b), buf.String())
		}
		f.Close()
	}
}

type errReader int

func (e errReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	var e errReader
	err := Convert(&buf, e, nil)
	if err == nil {
		t.Fatal("should be an error")
	}
}

func TestGuessLang(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(`
<pre>
def do_something():
  pass
</pre>
	`), &Option{
		GuessLang: func(s string) (string, error) { return "python", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "```python\ndef do_something():\n  pass\n```\n\n\n"
	if buf.String() != want {
		t.Errorf("\nwant:\n%s}}}\ngot:\n%s}}}\n", want, buf.String())
	}
}

func TestGuessLangFromClass(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(`
<pre><code class="foo bar language-python">def do_something():
  pass
</code></pre>
	`), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "```python\ndef do_something():\n  pass\n```\n\n\n"
	if buf.String() != want {
		t.Errorf("\nwant:\n%s}}}\ngot:\n%s}}}\n", want, buf.String())
	}
}

func TestGuessLangBq(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(`
<blockquote class="code">
<b>def</b> do_something():
  <i>pass</i>
</blockquote>
	`), &Option{
		GuessLang: func(s string) (string, error) { return "python", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "```python\ndef do_something():\n  pass\n```\n\n\n"
	if buf.String() != want {
		t.Errorf("\nwant:\n%s}}}\ngot:\n%s}}}\n", want, buf.String())
	}
}

func TestWhiteSpaceDelimiter(t *testing.T) {
	// Test adding delimiters only on the inner contents
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(
		`<strong> foo bar </strong>`,
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := " **foo bar** \n"
	if buf.String() != want {
		t.Errorf("\nwant:\n%q}}}\ngot:\n%q}}}\n", want, buf.String())
	}

	// Test that no delimiters are added if the contents is all whitespace
	var buf2 bytes.Buffer
	err = Convert(&buf2, strings.NewReader(
		`Hello<strong>  </strong>hi`,
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	want = "Hello hi\n"
	if buf2.String() != want {
		t.Errorf("\nwant:\n%q}}}\ngot:\n%q}}}\n", want, buf2.String())
	}

	// Test that line breaks are preserved even if delimiters are not added
	var buf3 bytes.Buffer
	err = Convert(&buf3, strings.NewReader(
		`<strong><br></strong>`,
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	want = "\n\n\n"
	if buf3.String() != want {
		t.Errorf("\nwant:\n%q}}}\ngot:\n%q}}}\n", want, buf3.String())
	}
}

func TestEmptyImageSrc(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(
		`<img src="" alt="foo bar">`,
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "\n"
	if buf.String() != want {
		t.Errorf("\nwant:\n%q}}}\ngot:\n%q}}}\n", want, buf.String())
	}
}

func TestBlockLink(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(
		`<a href="https://example.org"><img src="https://example.com/img" alt="foo bar"><div></div></a>`,
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "[![foo bar](https://example.com/img)](https://example.org)\n\n"
	if buf.String() != want {
		t.Errorf("\nwant:\n%q}}}\ngot:\n%q}}}\n", want, buf.String())
	}
}

func TestScript(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(`
<p>here is script</p>

<script type="text/javascript" src="https://code.jqeury.com/jquery-latest.js"></script>

<script type="text/javascript"><!--
alert(1)
--></script>
	`), &Option{
		Script: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `here is script

<script type="text/javascript" src="https://code.jqeury.com/jquery-latest.js"></script>

<script type="text/javascript"><!--
alert(1)
--></script>


`
	if buf.String() != want {
		t.Errorf("\nwant:\n%s}}}\ngot:\n%s}}}\n", want, buf.String())
	}
}

func TestStyle(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(`
<p>here is style</p>

<style><!--
body {
	background-color: red;
}
--></style>
	`), &Option{
		Style: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `here is style

<style><!--
body {
	background-color: red;
}
--></style>


`
	if buf.String() != want {
		t.Errorf("\nwant:\n%s}}}\ngot:\n%s}}}\n", want, buf.String())
	}
}

type TestRule struct{}

func (r *TestRule) Rule(next WalkFunc) (string, WalkFunc) {
	return "test", func(node *html.Node, w io.Writer, nest int, option *Option) {
		fmt.Fprint(w, "_")
		next(node, w, nest, option)
		fmt.Fprint(w, "_")
	}
}

func TestCustomRules(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(`
<test>here is the text in custom tag</test>
	`), &Option{
		CustomRules: []CustomRule{&TestRule{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `_here is the text in custom tag_
`
	if buf.String() != want {
		t.Errorf("\nwant:\n%s}}}\ngot:\n%s}}}\n", want, buf.String())
	}
}

type TestOverwriteRule struct{}

func (r *TestOverwriteRule) Rule(next WalkFunc) (string, WalkFunc) {
	return "div", func(node *html.Node, w io.Writer, nest int, option *Option) {
		fmt.Fprint(w, "___")
		next(node, w, nest, option)
		fmt.Fprint(w, "___")
	}
}

func TestCustomOverwriteRules(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(&buf, strings.NewReader(`
<div>here is the text in custom tag</div>
	`), &Option{
		CustomRules: []CustomRule{&TestOverwriteRule{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `___here is the text in custom tag___
`
	if buf.String() != want {
		t.Errorf("\nwant:\n%s}}}\ngot:\n%s}}}\n", want, buf.String())
	}
}

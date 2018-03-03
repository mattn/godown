package godown

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"
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
		if err = Convert(&buf, f); err != nil {
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
	err := Convert(&buf, e)
	if err == nil {
		t.Fatal("should be an error")
	}
}

package godown

import (
	"bytes"
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
	sort.Slice(m, func(i, j int) bool {
		return m[i] < m[j]
	})
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

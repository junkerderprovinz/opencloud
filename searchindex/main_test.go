package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

// newIndex builds a bleve index the way OpenCloud's search service does, with
// several batches so that the store holds more than one segment.
func newIndex(t *testing.T, path string) {
	t.Helper()
	index, err := bleve.New(path, bleve.NewIndexMapping())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		batch := index.NewBatch()
		for j := range 5 {
			id := fmt.Sprintf("doc-%d-%d", i, j)
			if err := batch.Index(id, map[string]string{"Name": id + ".txt"}); err != nil {
				t.Fatal(err)
			}
		}
		if err := index.Batch(batch); err != nil {
			t.Fatal(err)
		}
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
}

// opens reports whether bleve opens the index with the options the search
// service uses.
func opens(path string) bool {
	index, err := bleve.OpenUsing(path, map[string]any{"bolt_timeout": "5s"})
	if err != nil {
		return false
	}
	_ = index.Close()
	return true
}

func segments(t *testing.T, index string) []string {
	t.Helper()
	zaps, err := filepath.Glob(filepath.Join(index, "store", "*.zap"))
	if err != nil {
		t.Fatal(err)
	}
	if len(zaps) == 0 {
		t.Fatal("the index has no segment files")
	}
	return zaps
}

func TestDiagnoseAgreesWithBleve(t *testing.T) {
	damages := map[string]func(t *testing.T, index string){
		"intact": func(t *testing.T, index string) {},
		"newest segment removed": func(t *testing.T, index string) {
			zaps := segments(t, index)
			if err := os.Remove(zaps[len(zaps)-1]); err != nil {
				t.Fatal(err)
			}
		},
		"every segment removed": func(t *testing.T, index string) {
			for _, zap := range segments(t, index) {
				if err := os.Remove(zap); err != nil {
					t.Fatal(err)
				}
			}
		},
		"segment emptied": func(t *testing.T, index string) {
			zaps := segments(t, index)
			if err := os.Truncate(zaps[0], 0); err != nil {
				t.Fatal(err)
			}
		},
		"root.bolt removed": func(t *testing.T, index string) {
			if err := os.Remove(filepath.Join(index, "store", "root.bolt")); err != nil {
				t.Fatal(err)
			}
		},
		"root.bolt emptied": func(t *testing.T, index string) {
			if err := os.Truncate(filepath.Join(index, "store", "root.bolt"), 0); err != nil {
				t.Fatal(err)
			}
		},
		"index_meta.json removed": func(t *testing.T, index string) {
			if err := os.Remove(filepath.Join(index, "index_meta.json")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, damage := range damages {
		t.Run(name, func(t *testing.T) {
			index := filepath.Join(t.TempDir(), "bleve-v5")
			newIndex(t, index)
			damage(t, index)

			reason, err := diagnose(index)
			if err != nil {
				t.Fatal(err)
			}
			// diagnose goes first because opening a damaged index rewrites it
			if want := opens(index); (reason == "") != want {
				t.Fatalf("diagnose says %q, bleve opens it: %v", reason, want)
			}
		})
	}
}

func TestIndexTypeReadsBothMetaFormats(t *testing.T) {
	plain := `{"storage":"boltdb","index_type":"scorch"}`
	cases := map[string]string{
		plain:                                 "scorch",
		plain + "\x00\x00\x00\x00":            "scorch",
		"sealed" + "key" + "\x00\x00\x00\x03": "",
		"\x00\x00":                            "",
	}
	for meta, want := range cases {
		if got := indexType([]byte(meta)); got != want {
			t.Errorf("indexType(%q) = %q, want %q", meta, got, want)
		}
	}
}

func TestMoveBrokenSetsAsideOnlyTheBrokenIndex(t *testing.T) {
	dir := t.TempDir()
	healthy := filepath.Join(dir, "bleve")
	broken := filepath.Join(dir, "bleve-v5")
	newIndex(t, healthy)
	newIndex(t, broken)
	if err := os.Remove(filepath.Join(broken, "store", "root.bolt")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "bleve-v5.broken", "stale"), 0o755); err != nil {
		t.Fatal(err)
	}

	moved, err := moveBroken(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 1 || !strings.HasPrefix(moved[0], broken+": ") {
		t.Fatalf("moved %q, want only %s", moved, broken)
	}
	if _, err := os.Stat(broken); !os.IsNotExist(err) {
		t.Fatalf("%s is still in place", broken)
	}
	if _, err := os.Stat(filepath.Join(dir, "bleve-v5.broken", "stale")); !os.IsNotExist(err) {
		t.Fatal("the earlier broken copy was not replaced")
	}
	if !opens(healthy) {
		t.Fatal("the healthy index no longer opens")
	}
}

func TestMoveBrokenLeavesOtherDirectoriesAlone(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"bleve-v5.broken", "opensearch", "bleve-vx"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	moved, err := moveBroken(dir)
	if err != nil || len(moved) != 0 {
		t.Fatalf("moved %q, err %v", moved, err)
	}
}

func TestMoveBrokenWithoutSearchDirectory(t *testing.T) {
	moved, err := moveBroken(filepath.Join(t.TempDir(), "search"))
	if err != nil || len(moved) != 0 {
		t.Fatalf("moved %q, err %v", moved, err)
	}
}

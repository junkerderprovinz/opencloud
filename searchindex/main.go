// Command searchindex moves a bleve search index that OpenCloud cannot open out
// of the way. The search service then creates a fresh one, where the broken
// index would fail it five times and take the whole server down with it.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket and key names of bleve's scorch root.bolt.
var (
	snapshotsBucket = []byte{'s'}
	metaKey         = []byte{'m'}
	internalKey     = []byte{'i'}
	pathKey         = []byte{'p'}
	mappingKey      = []byte("_mapping")
)

// indexName matches the index directories OpenCloud creates: bleve up to 7.4,
// bleve-v<schema> after.
var indexName = regexp.MustCompile(`^bleve(-v[0-9]+)?$`)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: searchindex <search data directory>")
	}
	moved, err := moveBroken(os.Args[1])
	for _, line := range moved {
		fmt.Println(line)
	}
	if err != nil {
		log.Fatalf("searchindex: %v", err)
	}
}

// moveBroken renames every index under dir that bleve cannot open to
// <name>.broken, replacing an earlier one, and reports each move.
func moveBroken(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var moved []string
	for _, e := range entries {
		if !e.IsDir() || !indexName.MatchString(e.Name()) {
			continue
		}
		index := filepath.Join(dir, e.Name())
		reason, err := diagnose(index)
		if err != nil {
			return moved, fmt.Errorf("%s: %w", index, err)
		}
		if reason == "" {
			continue
		}
		aside := index + ".broken"
		if err := os.RemoveAll(aside); err != nil {
			return moved, err
		}
		if err := os.Rename(index, aside); err != nil {
			return moved, err
		}
		moved = append(moved, fmt.Sprintf("%s: %s, moved it to %s", index, reason, filepath.Base(aside)))
	}
	return moved, nil
}

// diagnose returns why bleve would fail to open the index, or "" when it would
// open it or the index is not one it can judge.
func diagnose(index string) (string, error) {
	meta, err := os.ReadFile(filepath.Join(index, "index_meta.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return "index_meta.json is missing", nil
	}
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(meta)) == 0 {
		return "index_meta.json is empty", nil
	}
	if indexType(meta) != "scorch" {
		return "", nil
	}

	store := filepath.Join(index, "store")
	root := filepath.Join(store, "root.bolt")
	// bbolt would try to initialise an empty file even when opened read-only
	info, err := os.Stat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return "store/root.bolt is missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.Size() == 0 {
		return "store/root.bolt is empty", nil
	}
	db, err := bolt.Open(root, 0o600, &bolt.Options{ReadOnly: true, Timeout: 5 * time.Second})
	if errors.Is(err, bolt.ErrTimeout) || errors.Is(err, fs.ErrPermission) {
		return "", err
	}
	if err != nil {
		return fmt.Sprintf("store/root.bolt cannot be read (%v)", err), nil
	}
	defer db.Close()

	var reason string
	err = db.View(func(tx *bolt.Tx) error {
		reason = latestSnapshot(tx.Bucket(snapshotsBucket), store)
		return nil
	})
	return reason, err
}

// indexType reads index_meta.json either as plain JSON or, as bleve 2.6 writes
// it, followed by an empty file callback id and its four-byte length. An index
// written through a real callback is encrypted and reads as "".
func indexType(meta []byte) string {
	var m struct {
		IndexType string `json:"index_type"`
	}
	if json.Unmarshal(meta, &m) == nil {
		return m.IndexType
	}
	n := len(meta) - 4
	if n < 0 || binary.BigEndian.Uint32(meta[n:]) != 0 || json.Unmarshal(meta[:n], &m) != nil {
		return ""
	}
	return m.IndexType
}

// latestSnapshot follows bleve's loadFromBolt: the newest snapshot whose
// segments all load becomes the index, and the mapping has to be in it.
func latestSnapshot(snapshots *bolt.Bucket, store string) string {
	if snapshots == nil {
		return "store/root.bolt holds no snapshot"
	}
	failed := ""
	c := snapshots.Cursor()
	for k, _ := c.Last(); k != nil; k, _ = c.Prev() {
		snapshot := snapshots.Bucket(k)
		if snapshot == nil || snapshot.Bucket(metaKey) == nil {
			continue
		}
		hasMapping, broken := loadable(snapshot, store)
		switch {
		case broken != "":
			if failed == "" {
				failed = broken
			}
		case !hasMapping:
			return "its newest readable snapshot has no index mapping"
		default:
			return ""
		}
	}
	if failed == "" {
		return "store/root.bolt holds no snapshot"
	}
	return failed
}

// loadable reports whether the snapshot carries the index mapping and why
// bleve could not load it, "" when it could.
func loadable(snapshot *bolt.Bucket, store string) (hasMapping bool, broken string) {
	c := snapshot.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		if k[0] == metaKey[0] {
			continue
		}
		b := snapshot.Bucket(k)
		if b == nil {
			return hasMapping, "a snapshot entry is not a bucket"
		}
		if k[0] == internalKey[0] {
			hasMapping = len(b.Get(mappingKey)) > 0
			continue
		}
		name := b.Get(pathKey)
		if len(name) == 0 {
			return hasMapping, "a segment has no file name"
		}
		info, err := os.Stat(filepath.Join(store, string(name)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return hasMapping, fmt.Sprintf("segment %s is missing or empty", name)
		}
	}
	return hasMapping, ""
}

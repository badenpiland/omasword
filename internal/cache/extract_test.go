package cache

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZip(t *testing.T, path, name, body string, corrupt bool) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if corrupt {
		// Flip bytes inside the compressed payload so decompression or
		// the CRC check fails part-way through.
		i := bytes.Index(data, []byte(name)) + len(name) + 8
		for j := i; j < i+16 && j < len(data); j++ {
			data[j] ^= 0xff
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractJSONFailureLeavesNoCacheFile(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "KJV.zip")
	writeZip(t, zipPath, "KJV.json", strings.Repeat(`{"verse":1,"text":"x"},`, 5000), true)

	c := &Cache{cacheDir: dir}
	if err := c.extractJSON(zipPath, "KJV"); err == nil {
		t.Fatal("expected extraction of a corrupt zip to fail")
	}
	if c.IsCached("KJV") {
		t.Fatal("a failed extraction must not leave KJV.json behind")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("temp files left in cache dir: %v", entries)
	}
}

func TestExtractJSONSuccess(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "KJV.zip")
	body := `[{"pk":1,"verse":1,"text":"In the beginning","book":1,"chapter":1}]`
	writeZip(t, zipPath, "KJV.json", body, false)

	c := &Cache{cacheDir: dir}
	if err := c.extractJSON(zipPath, "KJV"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "KJV.json"))
	if err != nil || string(got) != body {
		t.Fatalf("KJV.json = %q, %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only KJV.json, got %v", entries)
	}
}

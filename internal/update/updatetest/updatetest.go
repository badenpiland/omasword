// Package updatetest serves a fake GitHub release for tests. Nothing in it
// touches the network.
package updatetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omasword/internal/update"
)

// Release describes the fake release. Zero fields get working defaults.
type Release struct {
	Tag        string
	Binary     []byte // contents of omasword-linux-<arch> in the archive
	Arch       string // amd64 unless set
	Checksums  *string
	APIStatus  int    // HTTP status for the API call; 200 unless set
	AssetBase  string // overrides the download URL prefix in the API reply
	ArchiveRaw []byte // replaces the generated archive
}

// Serve starts the server and returns an Updater pointed at it.
func Serve(t *testing.T, r Release) *update.Updater {
	t.Helper()
	if r.Arch == "" {
		r.Arch = "amd64"
	}
	if r.Tag == "" {
		r.Tag = "v9.9.9"
	}
	name := "omasword-linux-" + r.Arch
	archive := r.ArchiveRaw
	if archive == nil {
		archive = TarGz(t, name, r.Binary)
	}
	sum := sha256.Sum256(archive)
	sums := hex.EncodeToString(sum[:]) + "  " + name + ".tar.gz\n"
	if r.Checksums != nil {
		sums = *r.Checksums
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	prefix := srv.URL + "/dl/"
	base := prefix
	if r.AssetBase != "" {
		base = r.AssetBase
	}
	mux.HandleFunc("/api/latest", func(w http.ResponseWriter, _ *http.Request) {
		if r.APIStatus != 0 && r.APIStatus != http.StatusOK {
			w.WriteHeader(r.APIStatus)
			return
		}
		assets := []map[string]string{
			{"name": name + ".tar.gz", "browser_download_url": base + name + ".tar.gz"},
		}
		if r.Checksums == nil || *r.Checksums != "" {
			assets = append(assets, map[string]string{"name": update.ChecksumsName, "browser_download_url": base + update.ChecksumsName})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": r.Tag, "assets": assets})
	})
	mux.HandleFunc("/dl/"+name+".tar.gz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(archive) })
	mux.HandleFunc("/dl/"+update.ChecksumsName, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(sums)) })
	return &update.Updater{
		APIURL:         srv.URL + "/api/latest",
		DownloadPrefix: prefix,
		Client:         &http.Client{Timeout: 5 * time.Second},
		Current:        "v0.3.0",
		GOOS:           "linux",
		GOARCH:         r.Arch,
	}
}

// TarGz builds a release-shaped archive holding one binary plus a README.
func TarGz(t *testing.T, name string, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		data []byte
		mode int64
	}{{"README.md", []byte("readme\n"), 0o644}, {name, binary, 0o755}} {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: f.mode, Size: int64(len(f.data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

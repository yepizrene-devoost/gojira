package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarArchive builds a gzip-compressed tar archive from the given members.
func tarArchive(t *testing.T, members ...tarMember) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, member := range members {
		header := &tar.Header{
			Name:     member.name,
			Mode:     0o755,
			Size:     int64(len(member.body)),
			Typeflag: member.typeflag,
			Linkname: member.linkname,
		}
		if member.typeflag == tar.TypeDir {
			header.Mode = 0o755
			header.Size = 0
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header %q: %v", member.name, err)
		}
		if member.typeflag == tar.TypeReg && len(member.body) > 0 {
			if _, err := tarWriter.Write(member.body); err != nil {
				t.Fatalf("write tar body %q: %v", member.name, err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}

type tarMember struct {
	name     string
	body     []byte
	typeflag byte
	linkname string
}

func regularMember(name, body string) tarMember {
	return tarMember{name: name, body: []byte(body), typeflag: tar.TypeReg}
}

// zipArchive builds a zip archive from the given members.
func zipArchive(t *testing.T, members map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range members {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func digestOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestArtifactNameMatchesReleaseTemplates(t *testing.T) {
	cases := []struct{ goos, goarch, want string }{
		{"linux", "amd64", "gojira_Linux_x86_64.tar.gz"},
		{"linux", "arm64", "gojira_Linux_arm64.tar.gz"},
		{"darwin", "amd64", "gojira_Darwin_x86_64.tar.gz"},
		{"darwin", "arm64", "gojira_Darwin_arm64.tar.gz"},
		{"windows", "amd64", "gojira_Windows_x86_64.zip"},
		{"windows", "arm64", "gojira_Windows_arm64.zip"},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"_"+tc.goarch, func(t *testing.T) {
			got, err := ArtifactName(tc.goos, tc.goarch)
			if err != nil {
				t.Fatalf("ArtifactName(%q, %q) returned unexpected error: %v", tc.goos, tc.goarch, err)
			}
			if got != tc.want {
				t.Fatalf("ArtifactName(%q, %q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
			}
		})
	}
}

func TestArtifactNameRejectsUnpublishedPlatforms(t *testing.T) {
	cases := [][2]string{
		{"freebsd", "amd64"},
		{"linux", "386"},
		{"linux", "riscv64"},
		{"plan9", "amd64"},
	}
	for _, tc := range cases {
		t.Run(tc[0]+"_"+tc[1], func(t *testing.T) {
			if _, err := ArtifactName(tc[0], tc[1]); err == nil {
				t.Fatalf("ArtifactName(%q, %q) accepted an unpublished platform", tc[0], tc[1])
			}
		})
	}
}

func TestFetchChecksumRequiresExactlyOneValidDigest(t *testing.T) {
	assetName := "gojira_Linux_x86_64.tar.gz"
	goodDigest := strings.Repeat("a", 64)

	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "selects our entry", body: strings.Repeat("b", 64) + "  other.tar.gz\n" + goodDigest + "  " + assetName + "\n"},
		{name: "missing entry", body: goodDigest + "  other.tar.gz\n", wantErr: true},
		{name: "duplicated entry", body: goodDigest + "  " + assetName + "\n" + strings.Repeat("c", 64) + "  " + assetName + "\n", wantErr: true},
		{name: "short digest", body: strings.Repeat("a", 63) + "  " + assetName + "\n", wantErr: true},
		{name: "non hex digest", body: strings.Repeat("z", 64) + "  " + assetName + "\n", wantErr: true},
		{name: "empty file", body: "", wantErr: true},
		{name: "no filename", body: goodDigest + "\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			digest, err := NewFetcher(server.Client()).FetchChecksum(context.Background(), server.URL, assetName)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("FetchChecksum accepted %q and returned %q", tc.name, digest)
				}
				return
			}
			if err != nil {
				t.Fatalf("FetchChecksum returned unexpected error: %v", err)
			}
			if digest != goodDigest {
				t.Fatalf("FetchChecksum = %q, want %q", digest, goodDigest)
			}
		})
	}
}

func TestDownloadVerifiedStagesTheExtractedBinary(t *testing.T) {
	binary := []byte("#!/bin/sh\necho gojira\n")
	archive := tarArchive(t, regularMember("README.md", "docs"), regularMember("gojira", string(binary)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	stageDir := t.TempDir()
	staged, err := NewFetcher(server.Client()).DownloadVerified(
		context.Background(), server.URL, "gojira_Linux_x86_64.tar.gz", digestOf(archive), stageDir)
	if err != nil {
		t.Fatalf("DownloadVerified returned unexpected error: %v", err)
	}
	if filepath.Dir(staged) != stageDir {
		t.Fatalf("staged path %q is outside the destination directory %q", staged, stageDir)
	}
	content, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("read staged binary: %v", err)
	}
	if !bytes.Equal(content, binary) {
		t.Fatalf("staged content = %q, want the archive binary", content)
	}
	info, err := os.Stat(staged)
	if err != nil {
		t.Fatalf("stat staged binary: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("staged mode = %o, want 0755", info.Mode().Perm())
	}
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		t.Fatalf("read stage dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("stage dir holds %d entries, want only the staged binary", len(entries))
	}
}

func TestDownloadVerifiedAcceptsWindowsZipArchives(t *testing.T) {
	binary := "MZ fake windows binary"
	archive := zipArchive(t, map[string]string{"README.md": "docs", "gojira.exe": binary})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	staged, err := NewFetcher(server.Client()).DownloadVerified(
		context.Background(), server.URL, "gojira_Windows_x86_64.zip", digestOf(archive), t.TempDir())
	if err != nil {
		t.Fatalf("DownloadVerified returned unexpected error: %v", err)
	}
	content, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("read staged binary: %v", err)
	}
	if string(content) != binary {
		t.Fatalf("staged content = %q, want the zip binary", content)
	}
}

func TestDownloadVerifiedRejectsUnverifiedDownloads(t *testing.T) {
	archive := tarArchive(t, regularMember("gojira", "binary"))
	cases := []struct {
		name       string
		archive    []byte
		digest     string
		statusCode int
	}{
		{name: "checksum mismatch", archive: archive, digest: strings.Repeat("d", 64), statusCode: http.StatusOK},
		{name: "http failure", archive: archive, digest: digestOf(archive), statusCode: http.StatusNotFound},
		{name: "empty digest", archive: archive, digest: "", statusCode: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write(tc.archive)
			}))
			defer server.Close()

			stageDir := t.TempDir()
			if _, err := NewFetcher(server.Client()).DownloadVerified(
				context.Background(), server.URL, "gojira_Linux_x86_64.tar.gz", tc.digest, stageDir); err == nil {
				t.Fatalf("DownloadVerified accepted %q", tc.name)
			}
			entries, err := os.ReadDir(stageDir)
			if err != nil {
				t.Fatalf("read stage dir: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("stage dir holds %d entries after a rejected download, want none", len(entries))
			}
		})
	}
}

func TestDownloadVerifiedRejectsUnsafeArchives(t *testing.T) {
	cases := []struct {
		name    string
		members []tarMember
		match   string
	}{
		{
			name:    "traversal entry",
			members: []tarMember{regularMember("gojira", "binary"), regularMember("../evil", "payload")},
			match:   "unsafe",
		},
		{
			name:    "absolute entry",
			members: []tarMember{regularMember("gojira", "binary"), regularMember("/etc/evil", "payload")},
			match:   "unsafe",
		},
		{
			name:    "symlink entry",
			members: []tarMember{regularMember("gojira", "binary"), {name: "link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"}},
			match:   "unsafe",
		},
		{
			name:    "no binary",
			members: []tarMember{regularMember("README.md", "docs")},
			match:   "gojira",
		},
		{
			name:    "two binaries",
			members: []tarMember{regularMember("gojira", "first"), regularMember("gojira", "second")},
			match:   "gojira",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			archive := tarArchive(t, tc.members...)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(archive)
			}))
			defer server.Close()

			stageDir := t.TempDir()
			_, err := NewFetcher(server.Client()).DownloadVerified(
				context.Background(), server.URL, "gojira_Linux_x86_64.tar.gz", digestOf(archive), stageDir)
			if err == nil {
				t.Fatalf("DownloadVerified accepted an archive with %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.match) {
				t.Fatalf("error %q does not explain %q", err, tc.match)
			}
			entries, err := os.ReadDir(stageDir)
			if err != nil {
				t.Fatalf("read stage dir: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("stage dir holds %d entries after a rejected archive, want none", len(entries))
			}
		})
	}
}

func TestDownloadVerifiedBoundsTheDownloadSize(t *testing.T) {
	archive := tarArchive(t, regularMember("gojira", strings.Repeat("x", 4096)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	fetcher := NewFetcher(server.Client())
	fetcher.maxBytes = 128

	if _, err := fetcher.DownloadVerified(
		context.Background(), server.URL, "gojira_Linux_x86_64.tar.gz", digestOf(archive), t.TempDir()); err == nil {
		t.Fatal("DownloadVerified accepted an archive above the size cap")
	}
}

func TestDownloadVerifiedRejectsTruncatedArchives(t *testing.T) {
	archive := tarArchive(t, regularMember("gojira", "binary"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive[:len(archive)-8])
	}))
	defer server.Close()

	stageDir := t.TempDir()
	if _, err := NewFetcher(server.Client()).DownloadVerified(
		context.Background(), server.URL, "gojira_Linux_x86_64.tar.gz", digestOf(archive), stageDir); err == nil {
		t.Fatal("DownloadVerified accepted a truncated archive")
	}
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		t.Fatalf("read stage dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("stage dir holds %d entries after a truncated archive, want none", len(entries))
	}
}

func TestFetchChecksumNameMentionsTheAsset(t *testing.T) {
	assetName := "gojira_Darwin_arm64.tar.gz"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  other_asset.tar.gz\n", strings.Repeat("e", 64))
	}))
	defer server.Close()

	_, err := NewFetcher(server.Client()).FetchChecksum(context.Background(), server.URL, assetName)
	if err == nil {
		t.Fatal("FetchChecksum accepted a checksums file without the asset")
	}
	if !strings.Contains(err.Error(), assetName) {
		t.Fatalf("error %q does not name the missing asset %q", err, assetName)
	}
}

package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	// DefaultMaxAssetBytes caps one release archive download, and also the
	// size of the extracted binary.
	DefaultMaxAssetBytes = 128 << 20

	maxChecksumBytes = 1 << 20

	binaryNameUnix    = "gojira"
	binaryNameWindows = "gojira.exe"
)

// Fetcher downloads and verifies release assets. It never writes outside the
// destination directory it is given and never buffers an archive in memory.
type Fetcher struct {
	http     *http.Client
	maxBytes int64
}

// NewFetcher returns a fetcher, falling back to a bounded default HTTP client.
func NewFetcher(httpClient *http.Client) *Fetcher {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	return &Fetcher{http: httpClient, maxBytes: DefaultMaxAssetBytes}
}

// FetchChecksum downloads a checksums file and returns the single SHA-256
// digest it declares for assetName. A missing, duplicated or malformed entry is
// an error: an upgrade never proceeds on an unverifiable download.
func (f *Fetcher) FetchChecksum(ctx context.Context, checksumsURL, assetName string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		return "", fmt.Errorf("build checksums request: %w", err)
	}
	response, err := f.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch checksums: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch checksums: unexpected HTTP status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxChecksumBytes))
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	return checksumFor(body, assetName)
}

func checksumFor(body []byte, assetName string) (string, error) {
	digest := ""
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return "", fmt.Errorf("checksums file is malformed: %q", line)
		}
		if strings.TrimPrefix(fields[1], "*") != assetName {
			continue
		}
		candidate := strings.ToLower(fields[0])
		if !isSHA256(candidate) {
			return "", fmt.Errorf("checksums file has a malformed digest for %s", assetName)
		}
		if digest != "" {
			return "", fmt.Errorf("checksums file has more than one entry for %s", assetName)
		}
		digest = candidate
	}
	if digest == "" {
		return "", fmt.Errorf("checksums file has no entry for %s", assetName)
	}
	return digest, nil
}

func isSHA256(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

// DownloadVerified streams an archive into stageDir, verifies its SHA-256
// digest against expectedDigest, and extracts the release binary as a staged
// 0755 file in the same directory. The returned path is not yet installed.
func (f *Fetcher) DownloadVerified(ctx context.Context, archiveURL, assetName, expectedDigest, stageDir string) (string, error) {
	if !isSHA256(strings.ToLower(expectedDigest)) {
		return "", fmt.Errorf("no valid checksum to verify %s", assetName)
	}
	archivePath, actualDigest, err := f.download(ctx, archiveURL, stageDir)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archivePath) }()

	if expected := strings.ToLower(expectedDigest); actualDigest != expected {
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, expected, actualDigest)
	}
	return f.extract(archivePath, assetName, stageDir)
}

func (f *Fetcher) download(ctx context.Context, archiveURL, stageDir string) (string, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("build download request: %w", err)
	}
	response, err := f.http.Do(request)
	if err != nil {
		return "", "", fmt.Errorf("download archive: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download archive: unexpected HTTP status %d", response.StatusCode)
	}

	archive, err := os.CreateTemp(stageDir, ".gojira.download.*")
	if err != nil {
		return "", "", fmt.Errorf("create download file: %w", err)
	}
	archivePath := archive.Name()
	cleanup := func() {
		_ = archive.Close()
		_ = os.Remove(archivePath)
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(archive, hasher), io.LimitReader(response.Body, f.maxBytes+1))
	if err != nil {
		cleanup()
		return "", "", fmt.Errorf("download archive: %w", err)
	}
	if written > f.maxBytes {
		cleanup()
		return "", "", fmt.Errorf("download archive: exceeds the %d byte limit", f.maxBytes)
	}
	if err := archive.Close(); err != nil {
		_ = os.Remove(archivePath)
		return "", "", fmt.Errorf("close download file: %w", err)
	}
	return archivePath, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (f *Fetcher) extract(archivePath, assetName, stageDir string) (string, error) {
	staged := ""
	fail := func(err error) (string, error) {
		if staged != "" {
			_ = os.Remove(staged)
		}
		return "", err
	}

	var err error
	switch {
	case strings.HasSuffix(assetName, ".zip"):
		staged, err = f.extractZip(archivePath, stageDir)
	case strings.HasSuffix(assetName, ".tar.gz"):
		staged, err = f.extractTarGz(archivePath, stageDir)
	default:
		return "", fmt.Errorf("unsupported archive %s", assetName)
	}
	if err != nil {
		return fail(err)
	}
	return staged, nil
}

func (f *Fetcher) extractTarGz(archivePath, stageDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	decompressed, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("read archive: %w", err)
	}
	defer func() { _ = decompressed.Close() }()

	staged := ""
	reader := tar.NewReader(decompressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if staged != "" {
				_ = os.Remove(staged)
			}
			return "", fmt.Errorf("read archive: %w", err)
		}
		if isUnsafeArchivePath(header.Name) {
			if staged != "" {
				_ = os.Remove(staged)
			}
			return "", fmt.Errorf("archive contains an unsafe path %q", header.Name)
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			if staged != "" {
				_ = os.Remove(staged)
			}
			return "", fmt.Errorf("archive contains an unsafe link entry %q", header.Name)
		}
		if header.Typeflag != tar.TypeReg || !isBinaryEntry(header.Name) {
			continue
		}
		if staged != "" {
			_ = os.Remove(staged)
			return "", errors.New("archive contains more than one gojira binary")
		}
		staged, err = f.writeStaged(stageDir, reader)
		if err != nil {
			return "", err
		}
	}
	if staged == "" {
		return "", errors.New("archive does not contain a gojira binary")
	}
	return staged, nil
}

func (f *Fetcher) extractZip(archivePath, stageDir string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("read archive: %w", err)
	}
	defer func() { _ = reader.Close() }()

	staged := ""
	for _, entry := range reader.File {
		if isUnsafeArchivePath(entry.Name) {
			if staged != "" {
				_ = os.Remove(staged)
			}
			return "", fmt.Errorf("archive contains an unsafe path %q", entry.Name)
		}
		if entry.Mode()&fs.ModeSymlink != 0 {
			if staged != "" {
				_ = os.Remove(staged)
			}
			return "", fmt.Errorf("archive contains an unsafe link entry %q", entry.Name)
		}
		if entry.FileInfo().IsDir() || !isBinaryEntry(entry.Name) {
			continue
		}
		if staged != "" {
			_ = os.Remove(staged)
			return "", errors.New("archive contains more than one gojira binary")
		}
		opened, err := entry.Open()
		if err != nil {
			return "", fmt.Errorf("read archive entry: %w", err)
		}
		staged, err = f.writeStaged(stageDir, opened)
		_ = opened.Close()
		if err != nil {
			return "", err
		}
	}
	if staged == "" {
		return "", errors.New("archive does not contain a gojira binary")
	}
	return staged, nil
}

// writeStaged writes at most maxBytes of reader into a new staged file and
// marks it executable.
func (f *Fetcher) writeStaged(stageDir string, reader io.Reader) (string, error) {
	staged, err := os.CreateTemp(stageDir, ".gojira.new.*")
	if err != nil {
		return "", fmt.Errorf("create staged binary: %w", err)
	}
	stagedPath := staged.Name()
	written, err := io.Copy(staged, io.LimitReader(reader, f.maxBytes+1))
	if err == nil && written > f.maxBytes {
		err = fmt.Errorf("extracted binary exceeds the %d byte limit", f.maxBytes)
	}
	if err == nil {
		err = staged.Chmod(0o755)
	}
	if closeErr := staged.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(stagedPath)
		return "", fmt.Errorf("write staged binary: %w", err)
	}
	return stagedPath, nil
}

// isBinaryEntry reports whether a root-level archive entry is the release
// binary. Metadata files shipped next to it are ignored.
func isBinaryEntry(name string) bool {
	cleaned := path.Clean(name)
	return cleaned == binaryNameUnix || cleaned == binaryNameWindows
}

func isUnsafeArchivePath(name string) bool {
	if name == "" {
		return true
	}
	cleaned := path.Clean(name)
	if strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return true
	}
	if filepath.IsAbs(name) || strings.ContainsRune(name, ':') {
		return true
	}
	return false
}

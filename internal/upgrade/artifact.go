package upgrade

import (
	"fmt"
	"runtime"
)

// ArtifactName returns the release asset name for a platform, mirroring the
// naming template in .goreleaser.yaml: gojira_<Os>_<arch>, with .zip on
// Windows and .tar.gz elsewhere.
func ArtifactName(goos, goarch string) (string, error) {
	osName, ok := artifactOS(goos)
	if !ok {
		return "", fmt.Errorf("unsupported platform %s/%s: no published release archive", goos, goarch)
	}
	archName, ok := artifactArch(goarch)
	if !ok {
		return "", fmt.Errorf("unsupported platform %s/%s: no published release archive", goos, goarch)
	}
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	return "gojira_" + osName + "_" + archName + extension, nil
}

// CurrentArtifactName returns the release asset name for the running platform.
func CurrentArtifactName() (string, error) {
	return ArtifactName(runtime.GOOS, runtime.GOARCH)
}

func artifactOS(goos string) (string, bool) {
	switch goos {
	case "linux":
		return "Linux", true
	case "darwin":
		return "Darwin", true
	case "windows":
		return "Windows", true
	default:
		return "", false
	}
}

func artifactArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "x86_64", true
	case "arm64":
		return "arm64", true
	default:
		return "", false
	}
}

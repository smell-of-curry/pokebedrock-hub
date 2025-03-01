package resources

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/schollz/progressbar/v3"
)

const (
	repoOwner = "smell-of-curry"
	repoName  = "pokebedrock-res"
	apiURL    = "https://api.github.com/repos/%s/%s/releases/latest"
)

// GithubRelease ...
type GithubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// EntityDefinition ...
type EntityDefinition struct {
	FormatVersion string `json:"format_version"`
	Client        struct {
		Description struct {
			Textures map[string]string `json:"textures"`
			Geometry map[string]string `json:"geometry"`
		} `json:"description"`
	} `json:"minecraft:client_entity"`
}

// Manager handles resource pack operations.
type Manager struct {
	log         *slog.Logger
	resourceDir string
}

// NewManager creates a new resource pack manager.
func NewManager(log *slog.Logger, resourceDir string) *Manager {
	return &Manager{
		log:         log,
		resourceDir: resourceDir,
	}
}

// UnpackedPath returns the path where the resource pack is unpacked.
func (m *Manager) UnpackedPath() string {
	return filepath.Join(m.resourceDir, "unpacked")
}

// CheckAndUpdate checks for updates and downloads if necessary.
func (m *Manager) CheckAndUpdate() error {
	// Ensure directory exists
	if err := os.MkdirAll(m.resourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create resource directory: %w", err)
	}

	// Get current version
	currentVersion := m.CurrentVersion()

	// Get latest release info
	release, err := m.LatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	if currentVersion == release.TagName {
		m.log.Info("Resource pack is up to date", "version", currentVersion)
		// Even if up to date, ensure it's unpacked
		packPath := filepath.Join(m.resourceDir, fmt.Sprintf("pokebedrock-res-%s.mcpack", currentVersion))
		return m.unzipResourcePack(packPath)
	}

	// Ask for update if there's an existing version
	if currentVersion != "" {
		update := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Resource pack update available (%s -> %s). Update now?", currentVersion, release.TagName),
			Default: true,
		}
		if err = survey.AskOne(prompt, &update); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !update {
			return nil
		}
	}

	// Download new version
	if err = m.downloadResourcePack(release); err != nil {
		return fmt.Errorf("failed to download resource pack: %w", err)
	}

	m.log.Info("Successfully updated resource pack", "tag-name", release.TagName)
	return nil
}

// CurrentVersion ...
func (m *Manager) CurrentVersion() string {
	files, err := os.ReadDir(m.resourceDir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "pokebedrock-res-") && strings.HasSuffix(file.Name(), ".mcpack") {
			version := strings.TrimPrefix(file.Name(), "pokebedrock-res-")
			version = strings.TrimSuffix(version, ".mcpack")
			return version
		}
	}
	return ""
}

// LatestRelease ...
func (m *Manager) LatestRelease() (*GithubRelease, error) {
	resp, err := http.Get(fmt.Sprintf(apiURL, repoOwner, repoName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release GithubRelease
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

// downloadResourcePack ...
func (m *Manager) downloadResourcePack(release *GithubRelease) error {
	if len(release.Assets) == 0 {
		return fmt.Errorf("no assets found in release")
	}

	resp, err := http.Get(release.Assets[0].BrowserDownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Clean up old files
	if err = os.RemoveAll(m.UnpackedPath()); err != nil {
		m.log.Warn("failed to clean up old unpacked files", "error", err)
	}

	// Create mcpack file
	packPath := filepath.Join(m.resourceDir, fmt.Sprintf("pokebedrock-res-%s.mcpack", release.TagName))
	out, err := os.Create(packPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Download the file
	if _, err = io.Copy(out, resp.Body); err != nil {
		return err
	}

	// Unzip the resource pack
	return m.unzipResourcePack(packPath)
}

// AlreadyUnpacked checks if the resource pack is already unpacked and matches the version.
func (m *Manager) AlreadyUnpacked(version string) bool {
	// Check if unpacked directory exists
	unpackPath := m.UnpackedPath()
	if _, err := os.Stat(unpackPath); os.IsNotExist(err) {
		return false
	}

	// Check version file
	versionFile := filepath.Join(unpackPath, ".version")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(content)) == version
}

// markAsUnpacked creates a version file in the unpacked directory.
func (m *Manager) markAsUnpacked(version string) error {
	versionFile := filepath.Join(m.UnpackedPath(), ".version")
	return os.WriteFile(versionFile, []byte(version), 0644)
}

// unzipResourcePack ...
func (m *Manager) unzipResourcePack(packPath string) error {
	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return fmt.Errorf("failed to open resource pack: %w", err)
	}
	defer reader.Close()

	// Get version from pack path
	version := strings.TrimPrefix(filepath.Base(packPath), "pokebedrock-res-")
	version = strings.TrimSuffix(version, ".mcpack")

	// Check if already unpacked with correct version
	if m.AlreadyUnpacked(version) {
		m.log.Info("Resource pack is already unpacked", "version", version)
		return nil
	}

	unpackPath := m.UnpackedPath()
	if err = os.MkdirAll(unpackPath, 0755); err != nil {
		return fmt.Errorf("failed to create unpack directory: %w", err)
	}

	// Count total files for progress bar
	totalFiles := 0
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			totalFiles++
		}
	}

	// Create progress bar
	bar := progressbar.Default(int64(totalFiles), "Unzipping resource pack")

	for _, file := range reader.File {
		path := filepath.Join(unpackPath, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open zip file: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return fmt.Errorf("failed to copy file contents: %w", err)
		}

		bar.Add(1)
	}

	// Mark as unpacked with current version
	if err = m.markAsUnpacked(version); err != nil {
		return fmt.Errorf("failed to mark as unpacked: %w", err)
	}

	return nil
}

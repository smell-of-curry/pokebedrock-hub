// Package resources provides a manager for resource packs.
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
	repoName  = "pokebedrock-hub-res"
	apiURL    = "https://api.github.com/repos/%s/%s/releases/latest"

	// DirectoryPermissions is the standard permission for creating directories
	directoryPermissions = 0755
)

// GithubRelease ...
type GithubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// ManifestJSON represents the structure of a Minecraft resource pack manifest.json file
type ManifestJSON struct {
	FormatVersion int `json:"format_version"`
	Header        struct {
		Name             string `json:"name"`
		Description      string `json:"description"`
		UUID             string `json:"uuid"`
		Version          []int  `json:"version"`
		MinEngineVersion []int  `json:"min_engine_version"`
	} `json:"header"`
	Modules []struct {
		Type        string `json:"type"`
		UUID        string `json:"uuid"`
		Version     []int  `json:"version"`
		Description string `json:"description,omitempty"`
	} `json:"modules"`
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
	if err := os.MkdirAll(m.resourceDir, directoryPermissions); err != nil {
		return fmt.Errorf("failed to create resource directory: %w", err)
	}

	// Get current version
	currentVersion, err := m.CurrentVersion()
	if err != nil {
		m.log.Info("No valid resource pack found, will download", "error", err)
	}

	// Get latest release info
	release, err := m.LatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	if currentVersion == strings.TrimPrefix(release.TagName, "v") && m.isResourcePackValid() {
		m.log.Info("Resource pack is up to date", "version", currentVersion)
		return nil
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

// CurrentVersion gets the current version of the resource pack.
// It tries to read from manifest.json if available, otherwise falls back to checking .mcpack files.
func (m *Manager) CurrentVersion() (string, error) {
	// First try to read from manifest.json in the unpacked directory
	manifestPath := filepath.Join(m.UnpackedPath(), "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		// Manifest exists, read version from it
		manifest, err := m.ReadManifest()
		if err == nil && len(manifest.Header.Version) >= 3 {
			// Convert version array to string format
			version := fmt.Sprintf("%d.%d.%d",
				manifest.Header.Version[0],
				manifest.Header.Version[1],
				manifest.Header.Version[2])
			return version, nil
		}
	}

	// Fall back to checking .mcpack files
	files, err := os.ReadDir(m.resourceDir)
	if err != nil {
		return "", fmt.Errorf("failed to read resource directory: %w", err)
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "pokebedrock-res-") && strings.HasSuffix(file.Name(), ".mcpack") {
			version := strings.TrimPrefix(file.Name(), "pokebedrock-res-")
			version = strings.TrimSuffix(version, ".mcpack")
			return version, nil
		}
	}

	return "", fmt.Errorf("no resource pack found")
}

// ReadManifest reads the manifest.json file from the unpacked resource pack
func (m *Manager) ReadManifest() (*ManifestJSON, error) {
	manifestPath := filepath.Join(m.UnpackedPath(), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest ManifestJSON
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// isResourcePackValid checks if the unpacked resource pack is valid by looking for manifest.json
func (m *Manager) isResourcePackValid() bool {
	manifestPath := filepath.Join(m.UnpackedPath(), "manifest.json")
	_, err := os.Stat(manifestPath)
	return err == nil
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

	// Download the file
	if _, err = io.Copy(out, resp.Body); err != nil {
		return err
	}

	// Unzip the resource pack
	if err = m.unzipResourcePack(packPath); err != nil {
		return err
	}

	out.Close()

	// Delete the .mcpack file after successful unpacking
	if err = os.Remove(packPath); err != nil {
		m.log.Warn("failed to delete .mcpack file after unpacking", "error", err)
	}

	return nil
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

// unzipResourcePack extracts the resource pack to the unpacked directory
func (m *Manager) unzipResourcePack(packPath string) error {
	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return fmt.Errorf("failed to open resource pack: %w", err)
	}
	defer reader.Close()

	// Get version from pack path for logging
	version := strings.TrimPrefix(filepath.Base(packPath), "pokebedrock-res-")
	version = strings.TrimSuffix(version, ".mcpack")

	// Check if unpacked directory exists
	unpackPath := m.UnpackedPath()
	if _, err := os.Stat(unpackPath); !os.IsNotExist(err) {
		// Delete the unpacked directory to ensure clean state
		if err := os.RemoveAll(unpackPath); err != nil {
			m.log.Warn("failed to clean up old unpacked directory", "error", err)
		}
	}

	if err := os.MkdirAll(unpackPath, directoryPermissions); err != nil {
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
		if strings.Contains(file.Name, "..") {
			m.log.Warn("skipping file with invalid path", "file", file.Name)
			continue
		}
		path := filepath.Join(unpackPath, file.Name)
		relPath, err := filepath.Rel(unpackPath, filepath.Clean(path))
		if err != nil || strings.HasPrefix(relPath, "..") {
			m.log.Warn("skipping file with invalid path", "file", file.Name)
			continue
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, directoryPermissions); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(path), directoryPermissions); err != nil {
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

		if err = bar.Add(1); err != nil {
			m.log.Warn("failed to update progress bar", "error", err)
		}
	}

	// Validate the unpacked resource pack
	if !m.isResourcePackValid() {
		return fmt.Errorf("unpacked resource pack is invalid: manifest.json not found")
	}

	m.log.Info("Successfully unpacked resource pack", "version", version)
	return nil
}

// Package resources provides a manager for resource packs.
package resources

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/schollz/progressbar/v3"
)

const (
	repoOwner   = "smell-of-curry"
	apiURL      = "https://api.github.com/repos/%s/%s/releases/latest"
	httpTimeout = 5 * time.Minute

	// DirectoryPermissions is the standard permission for creating directories
	directoryPermissions = 0755
)

type packSpec struct {
	owner string
	repo  string
	dir   string
}

var httpClient = &http.Client{Timeout: httpTimeout}

func httpGet(url string) (*http.Response, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "pokebedrock-hub/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return resp, cancel, nil
}

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
	packs       []packSpec
}

// NewManager creates a new resource pack manager.
func NewManager(log *slog.Logger, resourceDir string) *Manager {
	return &Manager{
		log:         log,
		resourceDir: resourceDir,
		packs: []packSpec{
			{owner: repoOwner, repo: "pokebedrock-hub-res", dir: "unpacked"},
			{owner: repoOwner, repo: "pokebedrock-res", dir: "pokebedrock-res"},
		},
	}
}

// UnpackedPath returns the path where the resource pack is unpacked.
func (m *Manager) UnpackedPath() string {
	if len(m.packs) == 0 {
		return filepath.Join(m.resourceDir, "unpacked")
	}

	return m.packPath(m.packs[0])
}

func (m *Manager) packPath(p packSpec) string {
	return filepath.Join(m.resourceDir, p.dir)
}

func (m *Manager) cleanupMcpacks(pack packSpec) {
	pattern := filepath.Join(m.resourceDir, fmt.Sprintf("%s-*.mcpack", pack.repo))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		m.log.Warn("failed to glob mcpack files", "pattern", pattern, "error", err)
		return
	}

	for _, path := range matches {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			m.log.Warn("failed to remove stale mcpack", "path", path, "error", removeErr)
		}
	}
}

// CheckAndUpdate checks for updates and downloads if necessary.
func (m *Manager) CheckAndUpdate() error {
	// Ensure directory exists
	if err := os.MkdirAll(m.resourceDir, directoryPermissions); err != nil {
		return fmt.Errorf("failed to create resource directory: %w", err)
	}

	for _, pack := range m.packs {
		m.cleanupMcpacks(pack)
		if err := m.checkAndUpdatePack(pack); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) checkAndUpdatePack(pack packSpec) error {
	packLog := m.log.With("pack", pack.repo)

	currentVersion, err := m.currentVersion(pack)
	if err != nil {
		packLog.Info("No valid resource pack found, will download", "error", err)
	}

	release, err := m.latestRelease(pack)
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	releaseVersion := strings.TrimPrefix(release.TagName, "v")

	if currentVersion == releaseVersion && m.isResourcePackValid(pack) {
		packLog.Info("Resource pack is up to date", "version", currentVersion)
		return nil
	}

	if currentVersion != "" {
		update := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf(
				"Resource pack %s update available (%s -> %s). Update now?",
				pack.repo, currentVersion, release.TagName,
			),
			Default: true,
		}
		if err = survey.AskOne(prompt, &update); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !update {
			return nil
		}
	}

	if err = m.downloadResourcePack(pack, release); err != nil {
		return fmt.Errorf("failed to download resource pack: %w", err)
	}

	packLog.Info("Successfully updated resource pack", "tag-name", release.TagName)
	return nil
}

// CurrentVersion gets the current version of the resource pack.
// It tries to read from manifest.json if available, otherwise falls back to checking .mcpack files.
func (m *Manager) CurrentVersion() (string, error) {
	if len(m.packs) == 0 {
		return "", fmt.Errorf("no resource packs configured")
	}

	return m.currentVersion(m.packs[0])
}

func (m *Manager) currentVersion(pack packSpec) (string, error) {
	manifestPath := filepath.Join(m.packPath(pack), "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		manifest, err := m.readManifest(pack)
		if err == nil && len(manifest.Header.Version) >= 3 {
			return fmt.Sprintf("%d.%d.%d",
				manifest.Header.Version[0],
				manifest.Header.Version[1],
				manifest.Header.Version[2]), nil
		}
	}

	versionFile := filepath.Join(m.packPath(pack), ".version")
	if data, err := os.ReadFile(versionFile); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	return "", fmt.Errorf("no resource pack found")
}

// ReadManifest reads the manifest.json file from the unpacked resource pack
func (m *Manager) ReadManifest() (*ManifestJSON, error) {
	if len(m.packs) == 0 {
		return nil, fmt.Errorf("no resource packs configured")
	}

	return m.readManifest(m.packs[0])
}

func (m *Manager) readManifest(pack packSpec) (*ManifestJSON, error) {
	manifestPath := filepath.Join(m.packPath(pack), "manifest.json")
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
func (m *Manager) isResourcePackValid(pack packSpec) bool {
	manifestPath := filepath.Join(m.packPath(pack), "manifest.json")
	_, err := os.Stat(manifestPath)
	return err == nil
}

// LatestRelease ...
func (m *Manager) LatestRelease() (*GithubRelease, error) {
	if len(m.packs) == 0 {
		return nil, fmt.Errorf("no resource packs configured")
	}

	return m.latestRelease(m.packs[0])
}

func (m *Manager) latestRelease(pack packSpec) (*GithubRelease, error) {
	resp, cancel, err := httpGet(fmt.Sprintf(apiURL, pack.owner, pack.repo))
	if err != nil {
		return nil, err
	}
	defer cancel()
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
func (m *Manager) downloadResourcePack(pack packSpec, release *GithubRelease) (err error) {
	packLog := m.log.With("pack", pack.repo)

	if len(release.Assets) == 0 {
		return fmt.Errorf("no assets found in release")
	}

	resp, cancel, err := httpGet(release.Assets[0].BrowserDownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer cancel()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	packPath := filepath.Join(m.resourceDir, fmt.Sprintf("%s-%s.mcpack", pack.repo, release.TagName))

	out, err := os.Create(packPath)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if removeErr := os.Remove(packPath); removeErr != nil {
				packLog.Warn("failed to clean up pack after error", "error", removeErr)
			}
		}
	}()

	if _, err = io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}

	if err = out.Close(); err != nil {
		return err
	}

	if err = m.unzipResourcePack(pack, packPath); err != nil {
		return err
	}

	if err = os.Remove(packPath); err != nil {
		packLog.Warn("failed to delete .mcpack file after unpacking", "error", err)
	}

	versionFile := filepath.Join(m.packPath(pack), ".version")
	releaseVersion := strings.TrimPrefix(release.TagName, "v")
	if err = os.WriteFile(versionFile, []byte(releaseVersion), 0600); err != nil {
		packLog.Warn("failed to write version file", "error", err)
	}

	return nil
}

// AlreadyUnpacked checks if the resource pack is already unpacked and matches the version.
func (m *Manager) AlreadyUnpacked(version string) bool {
	if len(m.packs) == 0 {
		return false
	}

	pack := m.packs[0]
	unpackPath := m.packPath(pack)
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
func (m *Manager) unzipResourcePack(pack packSpec, packPath string) error {
	packLog := m.log.With("pack", pack.repo)
	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return fmt.Errorf("failed to open resource pack: %w", err)
	}
	defer reader.Close()

	version := strings.TrimSuffix(filepath.Base(packPath), ".mcpack")
	if prefix := pack.repo + "-"; strings.HasPrefix(version, prefix) {
		version = strings.TrimPrefix(version, prefix)
	}

	unpackPath := m.packPath(pack)
	if err := os.RemoveAll(unpackPath); err != nil {
		packLog.Warn("failed to clean up old unpacked directory", "error", err)
	}

	if err := os.MkdirAll(unpackPath, directoryPermissions); err != nil {
		return fmt.Errorf("failed to create unpack directory: %w", err)
	}

	totalFiles := 0
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			totalFiles++
		}
	}

	bar := progressbar.Default(int64(totalFiles), fmt.Sprintf("Unzipping %s", pack.repo))

	for _, file := range reader.File {
		if strings.Contains(file.Name, "..") {
			packLog.Warn("skipping file with invalid path", "file", file.Name)

			continue
		}
		path := filepath.Join(unpackPath, file.Name)
		relPath, relErr := filepath.Rel(unpackPath, filepath.Clean(path))
		if relErr != nil || strings.HasPrefix(relPath, "..") {
			packLog.Warn("skipping file with invalid path", "file", file.Name)

			continue
		}

		if file.FileInfo().IsDir() {
			if mkErr := os.MkdirAll(path, directoryPermissions); mkErr != nil {
				return fmt.Errorf("failed to create directory: %w", mkErr)
			}

			continue
		}

		if dirErr := os.MkdirAll(filepath.Dir(path), directoryPermissions); dirErr != nil {
			return fmt.Errorf("failed to create directories: %w", dirErr)
		}

		outFile, fileErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if fileErr != nil {
			return fmt.Errorf("failed to create file: %w", fileErr)
		}

		rc, openErr := file.Open()
		if openErr != nil {
			outFile.Close()
			return fmt.Errorf("failed to open zip file: %w", openErr)
		}

		_, copyErr := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to copy file contents: %w", copyErr)
		}

		if addErr := bar.Add(1); addErr != nil {
			packLog.Warn("failed to update progress bar", "error", addErr)
		}
	}

	if !m.isResourcePackValid(pack) {
		return fmt.Errorf("unpacked resource pack is invalid: manifest.json not found")
	}

	packLog.Info("Successfully unpacked resource pack", "version", version)
	return nil
}

// FindFile searches all managed packs for the given relative path and returns the first match.
func (m *Manager) FindFile(relParts ...string) (string, error) {
	rel := filepath.Join(relParts...)
	var lastErr error

	for _, pack := range m.packs {
		candidate, err := m.FindFileInPack(pack.repo, relParts...)
		if err == nil {
			return candidate, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			lastErr = err
			continue
		}

		return "", err
	}

	if lastErr != nil {
		return "", lastErr
	}

	return "", fmt.Errorf("file %s not found in managed packs", rel)
}

// FindFileInPack searches a specific pack for the given relative path parts.
func (m *Manager) FindFileInPack(packName string, relParts ...string) (string, error) {
	rel := filepath.Join(relParts...)
	for _, pack := range m.packs {
		if pack.repo != packName {
			continue
		}

		candidate := filepath.Join(m.packPath(pack), rel)
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("file %s not found in pack %s: %w", rel, packName, err)
			}

			return "", fmt.Errorf("failed to stat file %s in pack %s: %w", rel, packName, err)
		}

		return candidate, nil
	}

	return "", fmt.Errorf("pack %s not managed", packName)
}

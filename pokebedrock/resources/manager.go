package resources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

const (
	repoOwner = "smell-of-curry"
	repoName  = "pokebedrock-res"
	apiURL    = "https://api.github.com/repos/%s/%s/releases/latest"
)

type GithubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Manager handles resource pack operations
type Manager struct {
	resourceDir string
}

// NewManager creates a new resource pack manager
func NewManager(resourceDir string) *Manager {
	return &Manager{resourceDir: resourceDir}
}

// CheckAndUpdate checks for updates and downloads if necessary
func (m *Manager) CheckAndUpdate() error {
	// Ensure directory exists
	if err := os.MkdirAll(m.resourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create resource directory: %w", err)
	}

	// Get current version
	currentVersion := m.getCurrentVersion()

	// Get latest release info
	release, err := m.getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	if currentVersion == release.TagName {
		fmt.Printf("Resource pack is up to date (%s)\n", currentVersion)
		return nil
	}

	// Ask for update if there's an existing version
	if currentVersion != "" {
		update := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Resource pack update available (%s -> %s). Update now?", currentVersion, release.TagName),
			Default: true,
		}
		if err := survey.AskOne(prompt, &update); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !update {
			return nil
		}
	}

	// Download new version
	if err := m.downloadResourcePack(release); err != nil {
		return fmt.Errorf("failed to download resource pack: %w", err)
	}

	// Clean up old version if it exists
	if currentVersion != "" {
		oldFile := filepath.Join(m.resourceDir, fmt.Sprintf("pokebedrock-res-%s.mcpack", currentVersion))
		if err := os.Remove(oldFile); err != nil {
			fmt.Printf("Warning: Failed to remove old resource pack: %v\n", err)
		}
	}

	fmt.Printf("Successfully updated resource pack to %s\n", release.TagName)
	return nil
}

func (m *Manager) getCurrentVersion() string {
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

func (m *Manager) getLatestRelease() (*GithubRelease, error) {
	resp, err := http.Get(fmt.Sprintf(apiURL, repoOwner, repoName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

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

	filename := filepath.Join(m.resourceDir, fmt.Sprintf("pokebedrock-res-%s.mcpack", release.TagName))
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// GiteaService provides Gitea API integration
type GiteaService struct {
	configService *ConfigService
}

func NewGiteaService(cs *ConfigService) *GiteaService {
	return &GiteaService{configService: cs}
}

// giteaCommit represents a commit from Gitea API
type giteaCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// GetLatestCommit returns the latest commit SHA and short info for a repo+branch.
// repoURL is the full git URL (e.g., http://gitea.example.com/org/repo.git)
func (s *GiteaService) GetLatestCommit(repoURL, branch string) (sha, message string, err error) {
	giteaURL, giteaToken, err := s.getConfig()
	if err != nil {
		return "", "", err
	}

	// Extract owner/repo from URL
	ownerRepo := extractOwnerRepo(repoURL, giteaURL)
	if ownerRepo == "" {
		return "", "", fmt.Errorf("cannot extract owner/repo from %s", repoURL)
	}

	apiURL := fmt.Sprintf("%s/api/v1/repos/%s/commits?sha=%s&limit=1",
		strings.TrimRight(giteaURL, "/"), ownerRepo, branch)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "token "+giteaToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("gitea request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("gitea API error %d: %s", resp.StatusCode, string(body))
	}

	var commits []giteaCommit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return "", "", fmt.Errorf("parse response error: %w", err)
	}

	if len(commits) == 0 {
		return "", "", fmt.Errorf("no commits found on branch %s", branch)
	}

	c := commits[0]
	sha = c.SHA
	if len(sha) > 8 {
		sha = sha[:8]
	}
	message = c.Commit.Message
	if len(message) > 200 {
		message = message[:200] + "..."
	}

	log.Printf("[gitea] latest commit on %s/%s: %s %s", ownerRepo, branch, sha, c.Commit.Message)
	return sha, message, nil
}

func (s *GiteaService) getConfig() (url, token string, err error) {
	url, err = s.configService.GetSysConfigValue("gitea", "url")
	if err != nil || url == "" {
		return "", "", fmt.Errorf("Gitea URL not configured")
	}
	token, err = s.configService.GetSysConfigValue("gitea", "token")
	if err != nil {
		return "", "", fmt.Errorf("Gitea token not configured")
	}
	return url, token, nil
}

// extractOwnerRepo extracts "owner/repo" from a git URL relative to the Gitea base URL.
func extractOwnerRepo(repoURL, giteaURL string) string {
	base := strings.TrimRight(giteaURL, "/")
	// Try to strip the base URL
	for _, prefix := range []string{base + "/", strings.Replace(base, "http://", "https://", 1) + "/", strings.Replace(base, "https://", "http://", 1) + "/"} {
		if strings.HasPrefix(repoURL, prefix) {
			path := strings.TrimPrefix(repoURL, prefix)
			path = strings.TrimSuffix(path, ".git")
			return path
		}
	}
	// Fallback: try to extract from URL path
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		repo := parts[len(parts)-1]
		owner := parts[len(parts)-2]
		repo = strings.TrimSuffix(repo, ".git")
		return owner + "/" + repo
	}
	return ""
}

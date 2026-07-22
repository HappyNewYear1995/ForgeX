package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"jenkinsAgent/config"
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
)

type JenkinsService struct {
	cfg             *config.JenkinsConfig
	client          *http.Client
	buildStore      *store.BuildStore
	sysStore        *store.SysConfigStore
	onBuildComplete func(*model.Build) // callback when build finishes
}

func NewJenkinsService(cfg *config.JenkinsConfig) *JenkinsService {
	jar, _ := cookiejar.New(nil)
	return &JenkinsService{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		buildStore: store.NewBuildStore(),
		sysStore:   store.NewSysConfigStore(),
	}
}

// getConfig returns Jenkins config, preferring DB values over yaml config.
func (s *JenkinsService) getConfig() (cfgURL, user, token string) {
	cfgURL = s.cfg.URL
	user = s.cfg.User
	token = s.cfg.Token

	if v, err := s.sysStore.Get("jenkins", "url"); err == nil && v != "" {
		cfgURL = v
	}
	if v, err := s.sysStore.Get("jenkins", "user"); err == nil && v != "" {
		user = v
	}
	if v, err := s.sysStore.Get("jenkins", "token"); err == nil && v != "" {
		token = v
	}
	cfgURL = strings.TrimRight(cfgURL, "/")
	return
}

type JenkinsBuildResponse struct {
	Number    int    `json:"number"`
	URL       string `json:"url"`
	Building  bool   `json:"building"`
	Result    string `json:"result"`
	Timestamp int64  `json:"timestamp"`
	Duration  int64  `json:"duration"`
}

// JenkinsCrumb holds the CSRF crumb from Jenkins.
type JenkinsCrumb struct {
	CrumbRequestField string `json:"crumbRequestField"`
	Crumb             string `json:"crumb"`
}

// getCrumb fetches the CSRF crumb from Jenkins.
func (s *JenkinsService) getCrumb() *JenkinsCrumb {
	cfgURL, cfgUser, cfgToken := s.getConfig()
	apiURL := cfgURL + "/crumbIssuer/api/json"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.SetBasicAuth(cfgUser, cfgToken)
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[jenkins] crumb fetch error: %v", err)
		return nil
	}
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var crumb JenkinsCrumb
	if err := json.NewDecoder(resp.Body).Decode(&crumb); err != nil {
		return nil
	}
	return &crumb
}

// applyCrumb adds the CSRF crumb header to a request.
func (s *JenkinsService) applyCrumb(req *http.Request) {
	crumb := s.getCrumb()
	if crumb != nil && crumb.Crumb != "" {
		req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	}
}

// ListJobs fetches all job names from Jenkins.
func (s *JenkinsService) ListJobs() ([]string, error) {
	cfgURL, cfgUser, cfgToken := s.getConfig()
	if cfgURL == "" {
		return nil, fmt.Errorf("Jenkins URL not configured")
	}
	apiURL := cfgURL + "/api/json?tree=jobs[name]"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cfgUser, cfgToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) { _ = Body.Close() }(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jenkins API returned status %d", resp.StatusCode)
	}
	var result struct {
		Jobs []struct {
			Name string `json:"name"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(result.Jobs))
	for _, j := range result.Jobs {
		names = append(names, j.Name)
	}
	return names, nil
}

// TriggerBuild triggers a Jenkins build with parameters and returns the build number.
func (s *JenkinsService) TriggerBuild(jobName string, params map[string]string) (int, error) {
	cfgURL, cfgUser, cfgToken := s.getConfig()

	// Try buildWithParameters first, fallback to build if job has no parameters
	buildURL := fmt.Sprintf("%s/job/%s/buildWithParameters", cfgURL, url.PathEscape(jobName))
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequest("POST", buildURL, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(cfgUser, cfgToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.applyCrumb(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// If 500 error, try /build (job may not have parameters)
	if resp.StatusCode == http.StatusInternalServerError {
		resp.Body.Close()
		buildURL = fmt.Sprintf("%s/job/%s/build", cfgURL, url.PathEscape(jobName))
		req2, err := http.NewRequest("POST", buildURL, nil)
		if err != nil {
			return 0, err
		}
		req2.SetBasicAuth(cfgUser, cfgToken)
		s.applyCrumb(req2)

		resp2, err := s.client.Do(req2)
		if err != nil {
			return 0, err
		}
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp2.Body)

		if resp2.StatusCode != http.StatusCreated && resp2.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("jenkins trigger failed: status=%d (tried both buildWithParameters and build)", resp2.StatusCode)
		}
		// Use Location header to resolve build number from queue
		if loc := resp2.Header.Get("Location"); loc != "" {
			if bn, err := s.resolveBuildFromQueue(loc, cfgUser, cfgToken); err == nil {
				return bn, nil
			}
		}
	} else if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("jenkins trigger failed: status=%d, body=%s", resp.StatusCode, string(body))
	} else {
		// Use Location header to resolve build number from queue
		if loc := resp.Header.Get("Location"); loc != "" {
			if bn, err := s.resolveBuildFromQueue(loc, cfgUser, cfgToken); err == nil {
				return bn, nil
			}
		}
	}

	// Fallback: wait and query lastBuild
	time.Sleep(2 * time.Second)
	return s.getLastBuildNumber(jobName)
}

// resolveBuildFromQueue polls the Jenkins queue item to get the actual build number.
func (s *JenkinsService) resolveBuildFromQueue(locationURL, cfgUser, cfgToken string) (int, error) {
	// Extract queue item URL: http://jenkins/queue/item/123/ -> http://jenkins/queue/item/123/api/json
	// Trim trailing slash and append /api/json
	queueAPI := strings.TrimRight(locationURL, "/") + "/api/json"

	// Poll up to 30 seconds for the build to be dequeued
	for i := 0; i < 15; i++ {
		if i > 0 {
			time.Sleep(2 * time.Second)
		}
		req, err := http.NewRequest("GET", queueAPI, nil)
		if err != nil {
			return 0, err
		}
		req.SetBasicAuth(cfgUser, cfgToken)

		resp, err := s.client.Do(req)
		if err != nil {
			continue
		}

		var result struct {
			Executable *struct {
				Number int `json:"number"`
			} `json:"executable"`
			Blocked  bool `json:"blocked"`
			Stuck    bool `json:"stuck"`
			Why      string `json:"why"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if result.Executable != nil && result.Executable.Number > 0 {
			return result.Executable.Number, nil
		}
		log.Printf("[jenkins] waiting for queue item to become executable (attempt %d, why=%s)", i+1, result.Why)
	}
	return 0, fmt.Errorf("queue item did not start within 30s")
}

func (s *JenkinsService) getLastBuildNumber(jobName string) (int, error) {
	cfgURL, cfgUser, cfgToken := s.getConfig()
	apiURL := fmt.Sprintf("%s/job/%s/lastBuild/api/json", cfgURL, url.PathEscape(jobName))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(cfgUser, cfgToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("jenkins query last build failed: status=%d", resp.StatusCode)
	}

	var result JenkinsBuildResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Number, nil
}

// GetBuildStatus queries Jenkins for the current status of a build.
func (s *JenkinsService) GetBuildStatus(jobName string, buildNumber int) (*JenkinsBuildResponse, error) {
	cfgURL, cfgUser, cfgToken := s.getConfig()
	apiURL := fmt.Sprintf("%s/job/%s/%d/api/json", cfgURL, url.PathEscape(jobName), buildNumber)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cfgUser, cfgToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jenkins query build failed: status=%d", resp.StatusCode)
	}

	var result JenkinsBuildResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DownloadFile downloads a file from Jenkins using authentication.
func (s *JenkinsService) DownloadFile(downloadURL string) (io.ReadCloser, string, error) {
	cfgURL, cfgUser, cfgToken := s.getConfig()
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.SetBasicAuth(cfgUser, cfgToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("download failed: status=%d", resp.StatusCode)
	}

	// Extract filename from URL or Content-Disposition
	filename := filepath.Base(downloadURL)
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if idx := strings.Index(cd, "filename="); idx >= 0 {
			filename = strings.Trim(cd[idx+9:], "\"' ")
		}
	}
	_ = cfgURL
	return resp.Body, filename, nil
}

// GetBuildLog fetches the console log for a specific build.
func (s *JenkinsService) GetBuildLog(jobName string, buildNumber int) (string, error) {
	cfgURL, cfgUser, cfgToken := s.getConfig()
	logURL := fmt.Sprintf("%s/job/%s/%d/consoleText", cfgURL, url.PathEscape(jobName), buildNumber)

	req, err := http.NewRequest("GET", logURL, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(cfgUser, cfgToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jenkins get log failed: status=%d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// TestConnection verifies Jenkins API connectivity.
func (s *JenkinsService) TestConnection() error {
	cfgURL, cfgUser, cfgToken := s.getConfig()
	if cfgURL == "" {
		return fmt.Errorf("Jenkins 地址未配置")
	}
	apiURL := cfgURL + "/api/json"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfgUser, cfgToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("认证失败 (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("异常状态码: %d", resp.StatusCode)
	}
	return nil
}

// StartPolling starts a background goroutine that polls pending/running builds.
func (s *JenkinsService) StartPolling(releaseService *ReleaseService) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.pollPendingBuilds(releaseService)
		}
	}()
	log.Println("[jenkins] build status polling started (interval=5s)")
}

// SetOnBuildComplete sets a callback that fires when a build completes.
func (s *JenkinsService) SetOnBuildComplete(fn func(*model.Build)) {
	s.onBuildComplete = fn
}

func (s *JenkinsService) pollPendingBuilds(releaseService *ReleaseService) {
	builds, err := s.buildStore.ListPending()
	if err != nil {
		log.Printf("[jenkins] poll error: %v", err)
		return
	}

	cfgURL, _, _ := s.getConfig()
	for _, build := range builds {
		if build.Product == nil || build.JenkinsBuildNumber == 0 {
			continue
		}

		// Determine job name
		jobName := build.JenkinsJobName
		if jobName == "" {
			jobName = build.Product.Name + "-pipeline"
		}

		status, err := s.GetBuildStatus(jobName, build.JenkinsBuildNumber)
		if err != nil {
			log.Printf("[jenkins] poll build#%d error: %v", build.ID, err)
			continue
		}

		if status.Building {
			if build.Status != model.BuildStatusRunning {
				build.Status = model.BuildStatusRunning
				now := time.Now()
				build.StartedAt = &now
				_ = s.buildStore.Update(&build)
			}
			// Check timeout: if running for more than 10 minutes without callback, mark as failed
			if build.StartedAt != nil && time.Since(*build.StartedAt) > 10*time.Minute {
				build.Status = model.BuildStatusFailed
				now := time.Now()
				build.FinishedAt = &now
				_ = s.buildStore.Update(&build)
				log.Printf("[jenkins] build#%d timed out after 10 minutes (no callback)", build.ID)

				// Handle failed release
				if build.ReleaseID > 0 && releaseService != nil {
					_ = releaseService.UpdateReleaseStatus(build.ReleaseID, model.ReleaseStatusFailed)
				}
			}
			continue
		}

		// Build completed
		switch status.Result {
		case "SUCCESS":
			build.Status = model.BuildStatusSuccess
		default:
			build.Status = model.BuildStatusFailed
		}
		now := time.Now()
		build.FinishedAt = &now
		build.LogURL = fmt.Sprintf("%s/job/%s/%d/console", cfgURL, url.PathEscape(jobName), build.JenkinsBuildNumber)
		_ = s.buildStore.Update(&build)

		// Update release status if linked
		if build.ReleaseID > 0 && releaseService != nil {
			if build.Status == model.BuildStatusSuccess {
				_ = releaseService.UpdateReleaseStatus(build.ReleaseID, model.ReleaseStatusReleased)
			} else {
				_ = releaseService.UpdateReleaseStatus(build.ReleaseID, model.ReleaseStatusFailed)
			}
		}

		log.Printf("[jenkins] build#%d completed: status=%s", build.ID, build.Status)

		// Trigger callback
		if s.onBuildComplete != nil {
			b := build // copy
			go s.onBuildComplete(&b)
		}
	}
}

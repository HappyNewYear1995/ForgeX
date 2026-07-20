package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	return &JenkinsService{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		buildStore: store.NewBuildStore(),
		sysStore:   store.NewSysConfigStore(),
	}
}

// getConfig returns Jenkins config, preferring DB values over yaml config.
func (s *JenkinsService) getConfig() (url, user, token string) {
	url = s.cfg.URL
	user = s.cfg.User
	token = s.cfg.Token

	if v, err := s.sysStore.Get("jenkins", "url"); err == nil && v != "" {
		url = v
	}
	if v, err := s.sysStore.Get("jenkins", "user"); err == nil && v != "" {
		user = v
	}
	if v, err := s.sysStore.Get("jenkins", "token"); err == nil && v != "" {
		token = v
	}
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

// TriggerBuild triggers a Jenkins build with parameters and returns the build number.
func (s *JenkinsService) TriggerBuild(jobName string, params map[string]string) (int, error) {
	cfgURL, cfgUser, cfgToken := s.getConfig()
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

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("jenkins trigger failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	time.Sleep(2 * time.Second)
	return s.getLastBuildNumber(jobName)
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
		jobName := build.Product.Name + "-pipeline"
		components, _ := store.NewComponentStore().ListByProductID(build.ProductID)
		if len(components) > 0 && components[0].JenkinsJobName != "" {
			jobName = components[0].JenkinsJobName
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

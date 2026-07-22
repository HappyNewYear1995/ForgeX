package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
	"jenkinsAgent/internal/utils"
)

type BuildService struct {
	buildStore      *store.BuildStore
	productStore    *store.ProductStore
	componentStore  *store.ComponentStore
	configItemStore *store.ConfigItemStore
	jenkinsService  *JenkinsService
	releaseService  *ReleaseService
	giteaService    *GiteaService
}

func NewBuildService(jenkins *JenkinsService, release *ReleaseService) *BuildService {
	return &BuildService{
		buildStore:      store.NewBuildStore(),
		productStore:    store.NewProductStore(),
		componentStore:  store.NewComponentStore(),
		configItemStore: store.NewConfigItemStore(),
		jenkinsService:  jenkins,
		releaseService:  release,
	}
}

func (s *BuildService) SetGiteaService(gs *GiteaService) {
	s.giteaService = gs
}

// BuildParams represents the parameters for triggering a build
type BuildParams struct {
	ProductID      uint
	ReleaseID      uint
	ProductVersion string
	BuildType      string // upgrade or full
	IsFormal       bool
	ReleaseNotes   string
	ComponentMods  string // JSON: [{"component_id":1,"component_code":"acis-svr","modules":[{"id":2,"code":"auth"}]}]
	AutoSyncTest   bool
	TriggeredBy    string
	RequestHost    string // host from HTTP request for callback URL generation
}

// compMod represents a component modification entry for build parameters
type compMod struct {
	ComponentID   uint   `json:"component_id"`
	ComponentCode string `json:"component_code"`
	Branch        string `json:"branch"`
	Modules       []struct {
		ID   uint   `json:"id"`
		Code string `json:"code"`
	} `json:"modules"`
}

// Trigger creates build record(s) and triggers Jenkins build(s)
func (s *BuildService) Trigger(params BuildParams) ([]*model.Build, error) {
	product, err := s.productStore.GetByID(params.ProductID)
	if err != nil {
		return nil, err
	}

	// Auto-calculate version if not specified
	if params.ProductVersion == "" {
		params.ProductVersion = utils.IncrementVersion(product.CurrentVersion)
	}

	// Parse componentMods
	var compModList []struct {
		ComponentID   uint   `json:"component_id"`
		ComponentCode string `json:"component_code"`
		Branch        string `json:"branch"`
		Modules       []struct {
			ID   uint   `json:"id"`
			Code string `json:"code"`
		} `json:"modules"`
	}
	if params.ComponentMods != "" {
		_ = json.Unmarshal([]byte(params.ComponentMods), &compModList)
	}

	// For full builds, use ALL components
	if params.BuildType == "full" {
		allMods := s.getAllComponentMods(params.ProductID)
		// Convert to match the compModList structure
		compModList = nil
		for _, m := range allMods {
			compModList = append(compModList, struct {
				ComponentID   uint   `json:"component_id"`
				ComponentCode string `json:"component_code"`
				Branch        string `json:"branch"`
				Modules       []struct {
					ID   uint   `json:"id"`
					Code string `json:"code"`
				} `json:"modules"`
			}{
				ComponentID:   m.ComponentID,
				ComponentCode: m.ComponentCode,
				Branch:        m.Branch,
			})
		}
	}

	// Auto-create Release for this build
	var release *model.Release
	if s.releaseService != nil {
		componentBranches := make(map[uint]string)
		for _, cm := range compModList {
			componentBranches[cm.ComponentID] = cm.Branch
		}
		rel, err := s.releaseService.CreateReleaseWithVersion(
			params.ProductID, params.ProductVersion, params.BuildType,
			params.ReleaseNotes, componentBranches, compModList,
		)
		if err != nil {
			log.Printf("[build] auto-create release error: %v", err)
		} else {
			release = rel
			params.ReleaseID = rel.ID
		}
	}

	// Fetch git commits from Gitea and update ReleaseComponent
	if release != nil && s.giteaService != nil {
		for _, cm := range compModList {
			comp, err := s.componentStore.GetByID(cm.ComponentID)
			if err != nil || comp == nil || comp.GitURL == "" {
				continue
			}
			branch := cm.Branch
			if branch == "" {
				branch = "main"
			}
			sha, msg, err := s.giteaService.GetLatestCommit(comp.GitURL, branch)
			if err != nil {
				log.Printf("[build] gitea commit error for %s/%s: %v", comp.Name, branch, err)
				continue
			}
			_ = s.releaseService.UpdateComponentSnapshot(release.ID, comp.ID, sha, msg, "pending")
		}
	}

	if product.JenkinsJobMode == "component" {
		return s.triggerComponentBuilds(product, params, release)
	}
	build, err := s.triggerProjectBuild(product, params, release)
	if err != nil {
		return nil, err
	}
	return []*model.Build{build}, nil
}

// triggerProjectBuild triggers a single project-level Jenkins job
func (s *BuildService) triggerProjectBuild(product *model.Product, params BuildParams, release *model.Release) (*model.Build, error) {
	jenkinsParams := s.buildJenkinsParams(params)
	callbackToken := generateToken()
	callbackURL := getCallbackURL(params.RequestHost, callbackToken)

	// Parse componentMods to get selected components
	var compModList []compMod
	if params.ComponentMods != "" {
		_ = json.Unmarshal([]byte(params.ComponentMods), &compModList)
	}

	// For full build, use ALL components; for upgrade, use selected only
	if params.BuildType == "full" {
		compModList = s.getAllComponentMods(params.ProductID)
	}

	// Build per-component parameters: {code}_REPO_URL, {code}_BRANCH, {code}_CALLBACK_URL, {code}_COMPONENT, {code}_MODULE_LIST
	for _, cm := range compModList {
		code := cm.ComponentCode
		if code == "" {
			code = s.getComponentCode(cm.ComponentID)
		}
		if code == "" {
			continue
		}
		repoURL, branch := s.getRepoInfo(cm.ComponentID, params.ComponentMods)
		jenkinsParams[code+"_REPO_URL"] = repoURL
		jenkinsParams[code+"_BRANCH"] = branch
		jenkinsParams[code+"_CALLBACK_URL"] = callbackURL
		jenkinsParams[code+"_COMPONENT"] = code
		// Build MODULE_LIST: use selected modules or load all for full build
		var moduleList []string
		for _, m := range cm.Modules {
			moduleList = append(moduleList, m.Code)
		}
		if len(moduleList) == 0 && params.BuildType == "full" {
			moduleList = s.getAllModuleCodes(cm.ComponentID)
		}
		jenkinsParams[code+"_MODULE_LIST"] = strings.Join(moduleList, ",")
	}

	jenkinsParams["CALLBACK_TOKEN"] = callbackToken
	paramsJSON, _ := json.Marshal(jenkinsParams)

	jobName := product.Name + "-pipeline"
	if product.JenkinsJobName != "" {
		jobName = product.JenkinsJobName
	}

	buildNumber, err := s.jenkinsService.TriggerBuild(jobName, jenkinsParams)
	if err != nil {
		return nil, fmt.Errorf("触发项目任务 %s 失败: %w", jobName, err)
	}

	build := &model.Build{
		ProductID:            params.ProductID,
		ReleaseID:            params.ReleaseID,
		JenkinsBuildNumber:   buildNumber,
		JenkinsJobName:       jobName,
		ProductVersion:       params.ProductVersion,
		BuildEnv:             params.BuildType,
		Status:               model.BuildStatusPending,
		TriggeredBy:          params.TriggeredBy,
		ReleaseNotes:         params.ReleaseNotes,
		ParametersJSON:       string(paramsJSON),
		RunScriptsAfterBuild: params.AutoSyncTest,
		CallbackToken:        callbackToken,
	}
	if err := s.buildStore.Create(build); err != nil {
		return nil, err
	}
	if params.ReleaseID > 0 && s.releaseService != nil {
		_ = s.releaseService.LinkBuild(params.ReleaseID, build.ID)
	}
	_ = release
	return build, nil
}

// triggerComponentBuilds triggers each component's Jenkins job in parallel
func (s *BuildService) triggerComponentBuilds(product *model.Product, params BuildParams, release *model.Release) ([]*model.Build, error) {
	components, err := s.componentStore.ListByProductID(params.ProductID)
	if err != nil {
		return nil, err
	}

	// Filter components that have a Jenkins job configured
	var jobComponents []model.Component
	for _, c := range components {
		if c.JenkinsJobName != "" {
			jobComponents = append(jobComponents, c)
		}
	}
	if len(jobComponents) == 0 {
		return nil, fmt.Errorf("没有配置 Jenkins 任务的组件")
	}

	// For upgrade build, only trigger selected components
	if params.BuildType == "upgrade" {
		var compModList []struct {
			ComponentID uint `json:"component_id"`
		}
		if params.ComponentMods != "" {
			_ = json.Unmarshal([]byte(params.ComponentMods), &compModList)
		}
		selectedIDs := make(map[uint]bool)
		for _, cm := range compModList {
			selectedIDs[cm.ComponentID] = true
		}
		var filtered []model.Component
		for _, c := range jobComponents {
			if selectedIDs[c.ID] {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("未选择任何组件")
		}
		jobComponents = filtered
	}
	// For full build, jobComponents already contains all components with Jenkins jobs

	type result struct {
		build *model.Build
		err   error
	}

	var wg sync.WaitGroup
	results := make([]result, len(jobComponents))

	for i, comp := range jobComponents {
		wg.Add(1)
		go func(idx int, c model.Component) {
			defer wg.Done()

			jp := s.buildJenkinsParams(params)
			jp["COMPONENT_NAME"] = c.Name
			// Get component code
			compCode := s.getComponentCode(c.ID)
			jp["COMPONENT"] = compCode
			// Add REPO_URL and BRANCH for this component
			repoURL, branch := s.getRepoInfo(c.ID, params.ComponentMods)
			if repoURL != "" {
				jp["REPO_URL"] = repoURL
			}
			if branch != "" {
				jp["BRANCH"] = branch
			}
			// Build MODULE_LIST from selected modules or all modules for full build
			var moduleList []string
			if params.ComponentMods != "" {
				var cmList []struct {
					ComponentID uint `json:"component_id"`
					Modules     []struct {
						Code string `json:"code"`
					} `json:"modules"`
				}
				_ = json.Unmarshal([]byte(params.ComponentMods), &cmList)
				for _, cm := range cmList {
					if cm.ComponentID == c.ID {
						for _, m := range cm.Modules {
							moduleList = append(moduleList, m.Code)
						}
						break
					}
				}
			}
			if len(moduleList) == 0 && params.BuildType == "full" {
				moduleList = s.getAllModuleCodes(c.ID)
			}
			jp["MODULE_LIST"] = strings.Join(moduleList, ",")
			callbackToken := generateToken()
			callbackURL := getCallbackURL(params.RequestHost, callbackToken)
			jp["CALLBACK_URL"] = callbackURL
			jp["CALLBACK_TOKEN"] = callbackToken
			paramsJSON, _ := json.Marshal(jp)

			buildNumber, triggerErr := s.jenkinsService.TriggerBuild(c.JenkinsJobName, jp)
			if triggerErr != nil {
				results[idx] = result{nil, fmt.Errorf("触发组件 %s 任务 %s 失败: %w", c.Name, c.JenkinsJobName, triggerErr)}
				return
			}

			b := &model.Build{
				ProductID:            params.ProductID,
				ReleaseID:            params.ReleaseID,
				JenkinsBuildNumber:   buildNumber,
				JenkinsJobName:       c.JenkinsJobName,
				ProductVersion:       params.ProductVersion,
				BuildEnv:             params.BuildType,
				Status:               model.BuildStatusPending,
				TriggeredBy:          params.TriggeredBy,
				ReleaseNotes:         params.ReleaseNotes,
				ParametersJSON:       string(paramsJSON),
				RunScriptsAfterBuild: params.AutoSyncTest,
				CallbackToken:        callbackToken,
			}
			if createErr := s.buildStore.Create(b); createErr != nil {
				results[idx] = result{nil, createErr}
				return
			}
			if params.ReleaseID > 0 && s.releaseService != nil {
				_ = s.releaseService.LinkBuild(params.ReleaseID, b.ID)
			}
			results[idx] = result{b, nil}
		}(i, comp)
	}
	wg.Wait()

	var builds []*model.Build
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		} else if r.build != nil {
			builds = append(builds, r.build)
		}
	}
	if len(errs) > 0 && len(builds) == 0 {
		return nil, fmt.Errorf("所有组件构建失败: %v", errs[0])
	}
	_ = release
	return builds, nil
}

// buildJenkinsParams creates the common Jenkins parameters map
func (s *BuildService) buildJenkinsParams(params BuildParams) map[string]string {
	return map[string]string{
		"PRODUCT_VERSION": params.ProductVersion,
		"BUILD_TYPE":      params.BuildType,
		"IS_FORMAL":       fmt.Sprintf("%t", params.IsFormal),
		"AUTO_SYNC_TEST":  fmt.Sprintf("%t", params.AutoSyncTest),
		"RELEASE_NOTES":   params.ReleaseNotes,
		"COMPONENT_MODS":  params.ComponentMods,
	}
}

// getRepoInfo extracts REPO_URL and BRANCH for a component from componentMods JSON
func (s *BuildService) getRepoInfo(compID uint, componentMods string) (repoURL, branch string) {
	var compModList []struct {
		ComponentID uint   `json:"component_id"`
		Branch      string `json:"branch"`
	}
	if componentMods != "" {
		_ = json.Unmarshal([]byte(componentMods), &compModList)
	}
	for _, cm := range compModList {
		if cm.ComponentID == compID {
			branch = cm.Branch
			break
		}
	}
	comp, err := s.componentStore.GetByID(compID)
	if err == nil && comp != nil {
		repoURL = comp.GitURL
	}
	return
}

// getComponentCode looks up the ConfigItem code for a component by its ID.
func (s *BuildService) getComponentCode(compID uint) string {
	comp, err := s.componentStore.GetByID(compID)
	if err != nil || comp == nil {
		return ""
	}
	item, err := s.configItemStore.GetByName(comp.Name)
	if err != nil || item == nil {
		return ""
	}
	return item.Code
}

// getAllComponentMods builds a compModList containing ALL components of a product.
func (s *BuildService) getAllComponentMods(productID uint) []compMod {
	components, _ := s.componentStore.ListByProductID(productID)
	var result []compMod
	for _, c := range components {
		code := ""
		if item, err := s.configItemStore.GetByName(c.Name); err == nil && item != nil {
			code = item.Code
		}
		branch := "main"
		if c.BranchFilter != "" {
			branch = c.BranchFilter
		}
		result = append(result, compMod{
			ComponentID:   c.ID,
			ComponentCode: code,
			Branch:        branch,
		})
	}
	return result
}

// getAllModuleCodes returns all module codes (children of component's ConfigItem).
func (s *BuildService) getAllModuleCodes(compID uint) []string {
	comp, err := s.componentStore.GetByID(compID)
	if err != nil || comp == nil {
		return nil
	}
	item, err := s.configItemStore.GetByName(comp.Name)
	if err != nil || item == nil {
		return nil
	}
	children, err := s.configItemStore.ListByParentID(item.ID)
	if err != nil {
		return nil
	}
	var codes []string
	for _, child := range children {
		if child.Code != "" {
			codes = append(codes, child.Code)
		}
	}
	return codes
}

func (s *BuildService) GetByID(id uint) (*model.Build, error) {
	return s.buildStore.GetByID(id)
}

func (s *BuildService) ListByProductID(productID uint) ([]model.Build, error) {
	return s.buildStore.ListByProductID(productID)
}

func (s *BuildService) ListRecent(limit int) ([]model.Build, error) {
	return s.buildStore.ListRecent(limit)
}

func (s *BuildService) DeleteBuild(id uint) error {
	return s.buildStore.Delete(id)
}

func (s *BuildService) Stats() (*store.BuildStats, error) {
	return s.buildStore.Stats()
}

func (s *BuildService) GetBuildLog(id uint) (string, error) {
	build, err := s.buildStore.GetByID(id)
	if err != nil {
		return "", err
	}
	jobName := build.JenkinsJobName
	if jobName == "" {
		product, err := s.productStore.GetByID(build.ProductID)
		if err != nil {
			return "", err
		}
		jobName = product.Name + "-pipeline"
	}
	return s.jenkinsService.GetBuildLog(jobName, build.JenkinsBuildNumber)
}

// GetArtifacts returns artifacts for a build
func (s *BuildService) GetArtifacts(buildID uint) ([]model.Artifact, error) {
	artifactStore := store.NewArtifactStore()
	return artifactStore.ListByBuildID(buildID)
}

func generateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getCallbackURL(host, token string) string {
	scheme := "http"
	// Use IP address instead of hostname to avoid DNS resolution issues
	ip := getLocalIP()
	// Extract port from host if present
	if _, port, err := net.SplitHostPort(host); err == nil && port != "" {
		return fmt.Sprintf("%s://%s:%s/api/callback/build/%s", scheme, ip, port, token)
	}
	return fmt.Sprintf("%s://%s/api/callback/build/%s", scheme, ip, token)
}

// getLocalIP returns the first non-loopback private IPv4 address of the machine.
// Private ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
			continue
		}
		if isPrivateIP(ipNet.IP) {
			return ipNet.IP.String()
		}
	}
	return "127.0.0.1"
}

// isPrivateIP checks if an IP is in a private/internal range.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

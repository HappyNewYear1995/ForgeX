package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
	"jenkinsAgent/internal/utils"
)

type BuildService struct {
	buildStore     *store.BuildStore
	productStore   *store.ProductStore
	componentStore *store.ComponentStore
	jenkinsService *JenkinsService
	releaseService *ReleaseService
}

func NewBuildService(jenkins *JenkinsService, release *ReleaseService) *BuildService {
	return &BuildService{
		buildStore:     store.NewBuildStore(),
		productStore:   store.NewProductStore(),
		componentStore: store.NewComponentStore(),
		jenkinsService: jenkins,
		releaseService: release,
	}
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

// Trigger creates a build record and triggers Jenkins build
func (s *BuildService) Trigger(params BuildParams) (*model.Build, error) {
	product, err := s.productStore.GetByID(params.ProductID)
	if err != nil {
		return nil, err
	}

	// Auto-calculate version if not specified
	if params.ProductVersion == "" {
		params.ProductVersion = utils.IncrementVersion(product.CurrentVersion)
	}

	// Parse componentMods to extract component IDs and branches
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

	// Build Jenkins parameters
	jenkinsParams := map[string]string{
		"PRODUCT_VERSION": params.ProductVersion,
		"BUILD_TYPE":      params.BuildType,
		"IS_FORMAL":       fmt.Sprintf("%t", params.IsFormal),
		"AUTO_SYNC_TEST":  fmt.Sprintf("%t", params.AutoSyncTest),
		"RELEASE_NOTES":   params.ReleaseNotes,
		"COMPONENT_MODS":  params.ComponentMods,
	}

	// Generate callback token for artifact upload
	callbackToken := generateToken()
	callbackURL := getCallbackURL(params.RequestHost, callbackToken)
	jenkinsParams["CALLBACK_URL"] = callbackURL
	jenkinsParams["CALLBACK_TOKEN"] = callbackToken

	paramsJSON, _ := json.Marshal(jenkinsParams)

	// Trigger Jenkins build (use product-level job or first component job)
	jobName := product.Name + "-pipeline"
	components, _ := s.componentStore.ListByProductID(params.ProductID)
	if len(components) > 0 && components[0].JenkinsJobName != "" {
		jobName = components[0].JenkinsJobName
	}

	buildNumber, err := s.jenkinsService.TriggerBuild(jobName, jenkinsParams)
	if err != nil {
		return nil, err
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

	build := &model.Build{
		ProductID:            params.ProductID,
		ReleaseID:            params.ReleaseID,
		JenkinsBuildNumber:   buildNumber,
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

	// Link build to release
	if params.ReleaseID > 0 && s.releaseService != nil {
		_ = s.releaseService.LinkBuild(params.ReleaseID, build.ID)
	}

	_ = release
	return build, nil
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

func (s *BuildService) Stats() (*store.BuildStats, error) {
	return s.buildStore.Stats()
}

func (s *BuildService) GetBuildLog(id uint) (string, error) {
	build, err := s.buildStore.GetByID(id)
	if err != nil {
		return "", err
	}
	product, err := s.productStore.GetByID(build.ProductID)
	if err != nil {
		return "", err
	}

	jobName := product.Name + "-pipeline"
	components, _ := s.componentStore.ListByProductID(build.ProductID)
	if len(components) > 0 && components[0].JenkinsJobName != "" {
		jobName = components[0].JenkinsJobName
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
	return fmt.Sprintf("%s://%s/api/callback/build/%s", scheme, host, token)
}

package service

import (
	"encoding/json"
	"fmt"
	"time"

	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
	"jenkinsAgent/internal/utils"
)

type ReleaseService struct {
	releaseStore   *store.ReleaseStore
	rcStore        *store.ReleaseComponentStore
	componentStore *store.ComponentStore
	productStore   *store.ProductStore
	buildStore     *store.BuildStore
}

func NewReleaseService() *ReleaseService {
	return &ReleaseService{
		releaseStore:   store.NewReleaseStore(),
		rcStore:        store.NewReleaseComponentStore(),
		componentStore: store.NewComponentStore(),
		productStore:   store.NewProductStore(),
		buildStore:     store.NewBuildStore(),
	}
}

// ManifestJSON represents the product manifest structure
type ManifestJSON struct {
	ProductInfo struct {
		Version     string `json:"version"`
		ReleaseDate string `json:"release_date"`
		BuildEnv    string `json:"build_env"`
		Description string `json:"description"`
	} `json:"product_info"`
	Components []ManifestComponent `json:"components"`
}

type ManifestComponent struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	GitBranch    string `json:"git_branch"`
	GitCommit    string `json:"git_commit"`
	ArtifactFile string `json:"artifact_file"`
	BuildStatus  string `json:"build_status"`
}

// CreateRelease creates a new release with component snapshots
func (s *ReleaseService) CreateRelease(productID uint, version, buildEnv, description string, componentBranches map[uint]string) (*model.Release, error) {
	product, err := s.productStore.GetByID(productID)
	if err != nil {
		return nil, err
	}

	release := &model.Release{
		ProductID:   productID,
		Version:     version,
		BuildEnv:    buildEnv,
		Description: description,
		Status:      model.ReleaseStatusDraft,
	}
	if err := s.releaseStore.Create(release); err != nil {
		return nil, err
	}

	// Create component snapshots
	components, _ := s.componentStore.ListByProductID(productID)
	for _, comp := range components {
		branch := "main"
		if b, ok := componentBranches[comp.ID]; ok && b != "" {
			branch = b
		}
		rc := &model.ReleaseComponent{
			ReleaseID:        release.ID,
			ComponentID:      comp.ID,
			ComponentName:    comp.Name,
			ComponentVersion: comp.CurrentVersion,
			GitBranch:        branch,
			GitCommit:        "", // filled after build
			ArtifactFile:     fmt.Sprintf("%s-v%s.tar.gz", comp.Name, version),
			BuildStatus:      "pending",
		}
		_ = s.rcStore.Create(rc)
	}

	// Update product current version
	product.CurrentVersion = version
	_ = s.productStore.Update(product)

	_ = product
	return release, nil
}

func (s *ReleaseService) GetRelease(id uint) (*model.Release, error) {
	return s.releaseStore.GetByID(id)
}

func (s *ReleaseService) ListReleases(productID uint) ([]model.Release, error) {
	return s.releaseStore.ListByProductID(productID)
}

func (s *ReleaseService) ListAllReleases(limit int) ([]model.Release, error) {
	return s.releaseStore.ListAll(limit)
}

func (s *ReleaseService) CountReleases() (int64, error) {
	return s.releaseStore.Count()
}

// UpdateReleaseStatus updates release status and generates manifest if completed
func (s *ReleaseService) UpdateReleaseStatus(releaseID uint, status model.ReleaseStatus) error {
	release, err := s.releaseStore.GetByID(releaseID)
	if err != nil {
		return err
	}
	release.Status = status

	if status == model.ReleaseStatusReleased {
		// Generate manifest JSON
		manifest := s.generateManifest(release)
		data, _ := json.MarshalIndent(manifest, "", "  ")
		release.ManifestJSON = string(data)
	}

	return s.releaseStore.Update(release)
}

// LinkBuild links a build record to a release
func (s *ReleaseService) LinkBuild(releaseID, buildID uint) error {
	release, err := s.releaseStore.GetByID(releaseID)
	if err != nil {
		return err
	}
	release.BuildID = buildID
	release.Status = model.ReleaseStatusBuilding
	return s.releaseStore.Update(release)
}

// UpdateComponentSnapshot updates git commit info for a release component
func (s *ReleaseService) UpdateComponentSnapshot(releaseID, componentID uint, gitCommit, buildStatus string) error {
	rcs, err := s.rcStore.ListByReleaseID(releaseID)
	if err != nil {
		return err
	}
	for _, rc := range rcs {
		if rc.ComponentID == componentID {
			rc.GitCommit = gitCommit
			rc.BuildStatus = buildStatus
			return s.rcStore.Update(&rc)
		}
	}
	return fmt.Errorf("component %d not found in release %d", componentID, releaseID)
}

func (s *ReleaseService) generateManifest(release *model.Release) *ManifestJSON {
	m := &ManifestJSON{}
	m.ProductInfo.Version = release.Version
	m.ProductInfo.ReleaseDate = release.CreatedAt.Format(time.RFC3339)
	m.ProductInfo.BuildEnv = release.BuildEnv
	m.ProductInfo.Description = release.Description

	components, _ := s.rcStore.ListByReleaseID(release.ID)
	for _, rc := range components {
		m.Components = append(m.Components, ManifestComponent{
			Name:         rc.ComponentName,
			Version:      rc.ComponentVersion,
			GitBranch:    rc.GitBranch,
			GitCommit:    rc.GitCommit,
			ArtifactFile: rc.ArtifactFile,
			BuildStatus:  rc.BuildStatus,
		})
	}
	return m
}

// CompModInfo represents parsed component info from build params
type CompModInfo struct {
	ComponentID   uint   `json:"component_id"`
	ComponentCode string `json:"component_code"`
	Branch        string `json:"branch"`
}

// CreateReleaseWithVersion creates a release and auto-increments component versions.
func (s *ReleaseService) CreateReleaseWithVersion(productID uint, version, buildEnv, description string, componentBranches map[uint]string, compModList interface{}) (*model.Release, error) {
	product, err := s.productStore.GetByID(productID)
	if err != nil {
		return nil, err
	}

	release := &model.Release{
		ProductID:   productID,
		Version:     version,
		BuildEnv:    buildEnv,
		Description: description,
		Status:      model.ReleaseStatusBuilding,
	}
	if err := s.releaseStore.Create(release); err != nil {
		return nil, err
	}

	// Extract component IDs from compModList
	type compMod struct {
		ComponentID   uint   `json:"component_id"`
		ComponentCode string `json:"component_code"`
		Branch        string `json:"branch"`
	}
	var mods []compMod
	if compModList != nil {
		data, _ := json.Marshal(compModList)
		_ = json.Unmarshal(data, &mods)
	}

	// Build component ID set
	compIDSet := make(map[uint]bool)
	compBranchMap := make(map[uint]string)
	for _, m := range mods {
		compIDSet[m.ComponentID] = true
		compBranchMap[m.ComponentID] = m.Branch
	}

	// Create component snapshots and auto-increment versions
	components, _ := s.componentStore.ListByProductID(productID)
	for _, comp := range components {
		if !compIDSet[comp.ID] {
			continue // skip components not in this build
		}

		branch := "main"
		if b, ok := compBranchMap[comp.ID]; ok && b != "" {
			branch = b
		} else if b, ok := componentBranches[comp.ID]; ok && b != "" {
			branch = b
		}

		// Auto-increment component version (DD + 1)
		newCompVersion := utils.IncrementVersion(comp.CurrentVersion)

		rc := &model.ReleaseComponent{
			ReleaseID:        release.ID,
			ComponentID:      comp.ID,
			ComponentName:    comp.Name,
			ComponentVersion: newCompVersion,
			GitBranch:        branch,
			GitCommit:        "",
			ArtifactFile:     fmt.Sprintf("%s-v%s.tar.gz", comp.Name, version),
			BuildStatus:      "pending",
		}
		_ = s.rcStore.Create(rc)

		// Update component's current version
		comp.CurrentVersion = newCompVersion
		_ = s.componentStore.Update(&comp)
	}

	// Update product current version
	product.CurrentVersion = version
	_ = s.productStore.Update(product)

	return release, nil
}

package store

import "jenkinsAgent/internal/model"

type BuildStore struct{}

func NewBuildStore() *BuildStore { return &BuildStore{} }

func (s *BuildStore) Create(b *model.Build) error {
	return DB.Create(b).Error
}

func (s *BuildStore) GetByID(id uint) (*model.Build, error) {
	var b model.Build
	err := DB.Preload("Product").Preload("Release").First(&b, id).Error
	return &b, err
}

func (s *BuildStore) GetByCallbackToken(token string) (*model.Build, error) {
	var b model.Build
	err := DB.Preload("Product").Preload("Release").Where("callback_token = ?", token).First(&b).Error
	return &b, err
}

func (s *BuildStore) ListByProductID(productID uint) ([]model.Build, error) {
	var builds []model.Build
	err := DB.Preload("Release").
		Where("product_id = ?", productID).
		Order("created_at desc").
		Find(&builds).Error
	return builds, err
}

func (s *BuildStore) ListRecent(limit int) ([]model.Build, error) {
	var builds []model.Build
	err := DB.Preload("Product").Preload("Release").
		Order("created_at desc").
		Limit(limit).
		Find(&builds).Error
	return builds, err
}

func (s *BuildStore) ListByReleaseID(releaseID uint) ([]model.Build, error) {
	var builds []model.Build
	err := DB.Preload("Release").
		Where("release_id = ?", releaseID).
		Order("created_at desc").
		Find(&builds).Error
	return builds, err
}

func (s *BuildStore) Update(b *model.Build) error {
	return DB.Save(b).Error
}

func (s *BuildStore) Delete(id uint) error {
	// Delete associated artifacts first
	DB.Where("build_id = ?", id).Delete(&model.Artifact{})
	return DB.Delete(&model.Build{}, id).Error
}

func (s *BuildStore) ListPending() ([]model.Build, error) {
	var builds []model.Build
	err := DB.Preload("Product").Preload("Release").
		Where("status IN ?", []model.BuildStatus{model.BuildStatusPending, model.BuildStatusRunning}).
		Find(&builds).Error
	return builds, err
}

type BuildStats struct {
	Total   int64
	Success int64
	Failed  int64
	Running int64
}

func (s *BuildStore) Stats() (*BuildStats, error) {
	var stats BuildStats
	DB.Model(&model.Build{}).Count(&stats.Total)
	DB.Model(&model.Build{}).Where("status = ?", model.BuildStatusSuccess).Count(&stats.Success)
	DB.Model(&model.Build{}).Where("status = ?", model.BuildStatusFailed).Count(&stats.Failed)
	DB.Model(&model.Build{}).Where("status IN ?", []model.BuildStatus{model.BuildStatusPending, model.BuildStatusRunning}).Count(&stats.Running)
	return &stats, nil
}

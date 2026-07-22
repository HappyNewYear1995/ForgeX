package store

import "jenkinsAgent/internal/model"

type ReleaseStore struct{}

func NewReleaseStore() *ReleaseStore { return &ReleaseStore{} }

func (s *ReleaseStore) Create(r *model.Release) error {
	return DB.Create(r).Error
}

func (s *ReleaseStore) GetByID(id uint) (*model.Release, error) {
	var r model.Release
	err := DB.Preload("Product").Preload("Components").Preload("Build").First(&r, id).Error
	return &r, err
}

func (s *ReleaseStore) ListByProductID(productID uint) ([]model.Release, error) {
	var releases []model.Release
	err := DB.Preload("Components").Preload("Build").
		Where("product_id = ?", productID).
		Order("created_at desc").
		Find(&releases).Error
	return releases, err
}

func (s *ReleaseStore) ListAll(limit int) ([]model.Release, error) {
	var releases []model.Release
	q := DB.Preload("Product").Preload("Components").Order("created_at desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&releases).Error
	return releases, err
}

func (s *ReleaseStore) Update(r *model.Release) error {
	return DB.Save(r).Error
}

func (s *ReleaseStore) Delete(id uint) error {
	// Delete associated release components first
	DB.Where("release_id = ?", id).Delete(&model.ReleaseComponent{})
	// Unlink build if associated
	DB.Model(&model.Build{}).Where("release_id = ?", id).Update("release_id", 0)
	return DB.Delete(&model.Release{}, id).Error
}

func (s *ReleaseStore) Count() (int64, error) {
	var count int64
	err := DB.Model(&model.Release{}).Count(&count).Error
	return count, err
}

// ReleaseComponentStore manages component snapshots within a release
type ReleaseComponentStore struct{}

func NewReleaseComponentStore() *ReleaseComponentStore { return &ReleaseComponentStore{} }

func (s *ReleaseComponentStore) Create(rc *model.ReleaseComponent) error {
	return DB.Create(rc).Error
}

func (s *ReleaseComponentStore) ListByReleaseID(releaseID uint) ([]model.ReleaseComponent, error) {
	var rcs []model.ReleaseComponent
	err := DB.Preload("Component").
		Where("release_id = ?", releaseID).
		Find(&rcs).Error
	return rcs, err
}

func (s *ReleaseComponentStore) Update(rc *model.ReleaseComponent) error {
	return DB.Save(rc).Error
}

func (s *ReleaseComponentStore) DeleteByReleaseID(releaseID uint) error {
	return DB.Where("release_id = ?", releaseID).Delete(&model.ReleaseComponent{}).Error
}

package store

import "jenkinsAgent/internal/model"

type ArtifactStore struct{}

func NewArtifactStore() *ArtifactStore { return &ArtifactStore{} }

func (s *ArtifactStore) Create(a *model.Artifact) error {
	return DB.Create(a).Error
}

func (s *ArtifactStore) ListByBuildID(buildID uint) ([]model.Artifact, error) {
	var artifacts []model.Artifact
	err := DB.Where("build_id = ?", buildID).Order("created_at desc").Find(&artifacts).Error
	return artifacts, err
}

func (s *ArtifactStore) GetByID(id uint) (*model.Artifact, error) {
	var a model.Artifact
	err := DB.Preload("Build").First(&a, id).Error
	return &a, err
}

func (s *ArtifactStore) Delete(id uint) error {
	return DB.Delete(&model.Artifact{}, id).Error
}

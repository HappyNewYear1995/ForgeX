package store

import "jenkinsAgent/internal/model"

type TestEnvScriptStore struct{}

func NewTestEnvScriptStore() *TestEnvScriptStore { return &TestEnvScriptStore{} }

func (s *TestEnvScriptStore) Create(item *model.TestEnvScript) error {
	return DB.Create(item).Error
}

func (s *TestEnvScriptStore) GetByID(id uint) (*model.TestEnvScript, error) {
	var item model.TestEnvScript
	err := DB.First(&item, id).Error
	return &item, err
}

func (s *TestEnvScriptStore) ListByTestEnvID(testEnvID uint) ([]model.TestEnvScript, error) {
	var items []model.TestEnvScript
	err := DB.Where("test_env_id = ?", testEnvID).Order("id ASC").Find(&items).Error
	return items, err
}

func (s *TestEnvScriptStore) Update(item *model.TestEnvScript) error {
	return DB.Save(item).Error
}

func (s *TestEnvScriptStore) Delete(id uint) error {
	return DB.Delete(&model.TestEnvScript{}, id).Error
}

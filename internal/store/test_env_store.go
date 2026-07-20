package store

import "jenkinsAgent/internal/model"

type TestEnvStore struct{}

func NewTestEnvStore() *TestEnvStore { return &TestEnvStore{} }

func (s *TestEnvStore) List() ([]model.TestEnv, error) {
	var items []model.TestEnv
	err := DB.Order("id ASC").Find(&items).Error
	return items, err
}

func (s *TestEnvStore) Create(item *model.TestEnv) error {
	return DB.Create(item).Error
}

func (s *TestEnvStore) Update(item *model.TestEnv) error {
	return DB.Save(item).Error
}

func (s *TestEnvStore) Delete(id uint) error {
	return DB.Delete(&model.TestEnv{}, id).Error
}

func (s *TestEnvStore) GetByID(id uint) (*model.TestEnv, error) {
	var item model.TestEnv
	err := DB.First(&item, id).Error
	return &item, err
}

// GetByProductID returns all test environments linked to a product via ProductTestEnv.
func (s *TestEnvStore) GetByProductID(productID uint) ([]model.TestEnv, error) {
	var ptes []model.ProductTestEnv
	if err := DB.Where("product_id = ?", productID).Find(&ptes).Error; err != nil {
		return nil, err
	}
	if len(ptes) == 0 {
		return nil, nil
	}
	envIDs := make([]uint, len(ptes))
	for i, pte := range ptes {
		envIDs[i] = pte.TestEnvID
	}
	var envs []model.TestEnv
	err := DB.Where("id IN ?", envIDs).Order("id ASC").Find(&envs).Error
	return envs, err
}

// SaveScript updates the script content for a test environment.
func (s *TestEnvStore) SaveScript(envID uint, content string) error {
	return DB.Model(&model.TestEnv{}).Where("id = ?", envID).Update("script_content", content).Error
}

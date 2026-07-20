package store

import "jenkinsAgent/internal/model"

type ConfigItemStore struct{}

func NewConfigItemStore() *ConfigItemStore { return &ConfigItemStore{} }

func (s *ConfigItemStore) List() ([]model.ConfigItem, error) {
	var items []model.ConfigItem
	err := DB.Order("sort_order, id").Find(&items).Error
	return items, err
}

func (s *ConfigItemStore) ListRoots() ([]model.ConfigItem, error) {
	var items []model.ConfigItem
	err := DB.Where("parent_id = 0").Order("sort_order, id").Find(&items).Error
	return items, err
}

func (s *ConfigItemStore) ListByParentID(parentID uint) ([]model.ConfigItem, error) {
	var items []model.ConfigItem
	err := DB.Where("parent_id = ?", parentID).Order("sort_order, id").Find(&items).Error
	return items, err
}

func (s *ConfigItemStore) GetByID(id uint) (*model.ConfigItem, error) {
	var item model.ConfigItem
	err := DB.First(&item, id).Error
	return &item, err
}

func (s *ConfigItemStore) Create(item *model.ConfigItem) error {
	return DB.Create(item).Error
}

func (s *ConfigItemStore) Update(item *model.ConfigItem) error {
	return DB.Save(item).Error
}

func (s *ConfigItemStore) Delete(id uint) error {
	// Delete children first
	DB.Where("parent_id = ?", id).Delete(&model.ConfigItem{})
	return DB.Delete(&model.ConfigItem{}, id).Error
}

func (s *ConfigItemStore) CountByParentID(parentID uint) (int64, error) {
	var count int64
	err := DB.Model(&model.ConfigItem{}).Where("parent_id = ?", parentID).Count(&count).Error
	return count, err
}

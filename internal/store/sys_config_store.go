package store

import "jenkinsAgent/internal/model"

type SysConfigStore struct{}

func NewSysConfigStore() *SysConfigStore { return &SysConfigStore{} }

// ListByCategory returns all configs for a given category.
func (s *SysConfigStore) ListByCategory(category string) ([]model.SysConfig, error) {
	var items []model.SysConfig
	err := DB.Where("category = ?", category).Order("id ASC").Find(&items).Error
	return items, err
}

// Get returns a single config value by category and key.
func (s *SysConfigStore) Get(category, key string) (string, error) {
	var item model.SysConfig
	err := DB.Where("category = ? AND key = ?", category, key).First(&item).Error
	if err != nil {
		return "", err
	}
	return item.Value, nil
}

// Upsert creates or updates a config entry.
func (s *SysConfigStore) Upsert(category, key, value, description string) error {
	var item model.SysConfig
	err := DB.Where("category = ? AND key = ?", category, key).First(&item).Error
	if err != nil {
		// Create new
		item = model.SysConfig{
			Category:    category,
			Key:         key,
			Value:       value,
			Description: description,
		}
		return DB.Create(&item).Error
	}
	// Update existing
	item.Value = value
	if description != "" {
		item.Description = description
	}
	return DB.Save(&item).Error
}

// SaveBatch saves multiple key-value pairs for a category.
func (s *SysConfigStore) SaveBatch(category string, pairs map[string]string) error {
	tx := DB.Begin()
	for key, value := range pairs {
		var item model.SysConfig
		err := tx.Where("category = ? AND key = ?", category, key).First(&item).Error
		if err != nil {
			item = model.SysConfig{Category: category, Key: key, Value: value}
			if err := tx.Create(&item).Error; err != nil {
				tx.Rollback()
				return err
			}
		} else {
			item.Value = value
			if err := tx.Save(&item).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit().Error
}

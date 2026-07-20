package store

import "jenkinsAgent/internal/model"

type ComponentStore struct{}

func NewComponentStore() *ComponentStore { return &ComponentStore{} }

func (s *ComponentStore) Create(c *model.Component) error {
	return DB.Create(c).Error
}

func (s *ComponentStore) GetByID(id uint) (*model.Component, error) {
	var c model.Component
	err := DB.Preload("Product").First(&c, id).Error
	return &c, err
}

func (s *ComponentStore) ListByProductID(productID uint) ([]model.Component, error) {
	var components []model.Component
	err := DB.Where("product_id = ?", productID).Order("created_at desc").Find(&components).Error
	return components, err
}

func (s *ComponentStore) Update(c *model.Component) error {
	return DB.Save(c).Error
}

func (s *ComponentStore) Delete(id uint) error {
	return DB.Delete(&model.Component{}, id).Error
}

func (s *ComponentStore) Count() (int64, error) {
	var count int64
	err := DB.Model(&model.Component{}).Count(&count).Error
	return count, err
}

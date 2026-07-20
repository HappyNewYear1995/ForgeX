package store

import "jenkinsAgent/internal/model"

type ProductStore struct{}

func NewProductStore() *ProductStore { return &ProductStore{} }

func (s *ProductStore) Create(p *model.Product) error {
	return DB.Create(p).Error
}

func (s *ProductStore) GetByID(id uint) (*model.Product, error) {
	var p model.Product
	err := DB.First(&p, id).Error
	return &p, err
}

func (s *ProductStore) List(keyword string) ([]model.Product, error) {
	var products []model.Product
	q := DB.Order("created_at desc")
	if keyword != "" {
		q = q.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := q.Find(&products).Error
	return products, err
}

func (s *ProductStore) Update(p *model.Product) error {
	return DB.Save(p).Error
}

func (s *ProductStore) Delete(id uint) error {
	DB.Where("product_id = ?", id).Delete(&model.Build{})
	DB.Where("product_id = ?", id).Delete(&model.Release{})
	DB.Where("product_id = ?", id).Delete(&model.Component{})
	return DB.Delete(&model.Product{}, id).Error
}

func (s *ProductStore) Count() (int64, error) {
	var count int64
	err := DB.Model(&model.Product{}).Count(&count).Error
	return count, err
}

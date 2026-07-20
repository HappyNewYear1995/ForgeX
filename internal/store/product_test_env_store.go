package store

import (
	"jenkinsAgent/internal/model"

	"gorm.io/gorm"
)

type ProductTestEnvStore struct{}

func NewProductTestEnvStore() *ProductTestEnvStore { return &ProductTestEnvStore{} }

// ListByProductID returns all test env associations for a product.
func (s *ProductTestEnvStore) ListByProductID(productID uint) ([]model.ProductTestEnv, error) {
	var items []model.ProductTestEnv
	err := DB.Where("product_id = ?", productID).Find(&items).Error
	return items, err
}

// Replace replaces all test env associations for a product with the given test env IDs.
func (s *ProductTestEnvStore) Replace(productID uint, testEnvIDs []uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// Delete existing
		if err := tx.Where("product_id = ?", productID).Delete(&model.ProductTestEnv{}).Error; err != nil {
			return err
		}
		// Insert new
		for _, envID := range testEnvIDs {
			pte := &model.ProductTestEnv{
				ProductID: productID,
				TestEnvID: envID,
			}
			if err := tx.Create(pte).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteByProductID removes all associations for a product.
func (s *ProductTestEnvStore) DeleteByProductID(productID uint) error {
	return DB.Where("product_id = ?", productID).Delete(&model.ProductTestEnv{}).Error
}

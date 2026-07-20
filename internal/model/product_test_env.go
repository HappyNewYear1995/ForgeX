package model

import "time"

// ProductTestEnv is a many-to-many junction table linking Product to TestEnv.
type ProductTestEnv struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"index;not null" json:"product_id"`
	TestEnvID uint      `gorm:"index;not null" json:"test_env_id"`
	CreatedAt time.Time `json:"created_at"`
}

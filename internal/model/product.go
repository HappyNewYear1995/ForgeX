package model

import "time"

// Product represents L1 - 产品版本 (Product Release)
type Product struct {
	ID             uint        `gorm:"primaryKey" json:"id"`
	Name           string      `gorm:"size:128;not null;uniqueIndex" json:"name"`
	Description    string      `gorm:"size:512" json:"description"`
	CurrentVersion string      `gorm:"size:64;default:0.0.0.0" json:"current_version"`
	TestEnvEnabled bool        `gorm:"default:false" json:"test_env_enabled"`
	Components     []Component `gorm:"foreignKey:ProductID" json:"components,omitempty"`
	Releases       []Release   `gorm:"foreignKey:ProductID" json:"releases,omitempty"`
	Builds         []Build     `gorm:"foreignKey:ProductID" json:"builds,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

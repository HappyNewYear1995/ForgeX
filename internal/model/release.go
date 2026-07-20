package model

import "time"

type ReleaseStatus string

const (
	ReleaseStatusDraft    ReleaseStatus = "draft"
	ReleaseStatusBuilding ReleaseStatus = "building"
	ReleaseStatusReleased ReleaseStatus = "released"
	ReleaseStatusFailed   ReleaseStatus = "failed"
)

// Release represents L1 版本发布 / Manifest 清单
type Release struct {
	ID             uint               `gorm:"primaryKey" json:"id"`
	ProductID      uint               `gorm:"index;not null" json:"product_id"`
	Version        string             `gorm:"size:64;not null" json:"version"`
	BuildID        uint               `json:"build_id"`
	BuildEnv       string             `gorm:"size:32;default:dev" json:"build_env"`
	Description    string             `gorm:"size:1024" json:"description"`
	Status         ReleaseStatus      `gorm:"size:32;default:draft" json:"status"`
	ManifestJSON   string             `gorm:"type:text" json:"manifest_json"`
	ArtifactURL    string             `gorm:"size:512" json:"artifact_url"`
	Product        *Product           `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Build          *Build             `gorm:"foreignKey:BuildID" json:"build,omitempty"`
	Components     []ReleaseComponent `gorm:"foreignKey:ReleaseID" json:"components,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

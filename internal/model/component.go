package model

import "time"

type ComponentType string

const (
	ComponentTypeFrontend ComponentType = "frontend"
	ComponentTypeBackend  ComponentType = "backend"
	ComponentTypeOther    ComponentType = "other"
)

// Component represents L2 - 组件版本 (Component Version)
type Component struct {
	ID             uint          `gorm:"primaryKey" json:"id"`
	ProductID      uint          `gorm:"index;not null" json:"product_id"`
	Name           string        `gorm:"size:128;not null" json:"name"`
	Type           ComponentType `gorm:"size:32;default:backend" json:"type"`
	GitURL         string        `gorm:"size:512" json:"git_url"`
	BranchFilter   string        `gorm:"size:256" json:"branch_filter"`
	JenkinsJobName string        `gorm:"size:256" json:"jenkins_job_name"`
	CurrentVersion string        `gorm:"size:64;default:0.0.0.0" json:"current_version"`
	Product        *Product      `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

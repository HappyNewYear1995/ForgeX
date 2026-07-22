package model

import "time"

// ConfigItem represents a node in the configuration dictionary tree.
// Root nodes have ParentID=0. Children represent modules under a component.
type ConfigItem struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	ParentID    uint            `gorm:"index;default:0" json:"parent_id"`
	Name        string          `gorm:"size:128;not null" json:"name"`
	Code        string          `gorm:"size:128;not null;uniqueIndex" json:"code"`
	Type        ComponentType   `gorm:"size:32;default:backend" json:"type"`
	Description string          `gorm:"size:512" json:"description"`
	SortOrder   int             `gorm:"default:0" json:"sort_order"`
	Children    []ConfigItem    `gorm:"-" json:"children,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

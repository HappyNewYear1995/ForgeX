package model

import "time"

// SysConfig stores system-level key-value configuration (Jenkins, Gitea, etc.)
type SysConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Category    string    `gorm:"size:64;not null;index:idx_sys_cat_key,unique" json:"category"`
	Key         string    `gorm:"size:128;not null;index:idx_sys_cat_key,unique" json:"key"`
	Value       string    `gorm:"size:1024" json:"value"`
	Description string    `gorm:"size:256" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

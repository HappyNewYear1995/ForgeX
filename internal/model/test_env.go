package model

import "time"

// TestEnv represents a test environment (HTTP service endpoint).
type TestEnv struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Name          string     `gorm:"size:128;not null" json:"name"`
	URL           string     `gorm:"size:512;not null" json:"url"`
	ScriptContent string     `gorm:"type:text" json:"script_content"`
	LastRunStatus string     `gorm:"size:32;default:idle" json:"last_run_status"` // idle, running, success, failed
	LastRunOutput string     `gorm:"type:text" json:"last_run_output"`
	LastRunAt     *time.Time `json:"last_run_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

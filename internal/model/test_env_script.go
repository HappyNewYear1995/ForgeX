package model

import "time"

// TestEnvScript represents a Playwright automation script for a test environment.
type TestEnvScript struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TestEnvID     uint       `gorm:"index;not null" json:"test_env_id"`
	Name          string     `gorm:"size:128;not null" json:"name"`
	ScriptContent string     `gorm:"type:text" json:"script_content"`
	LastRunStatus string     `gorm:"size:32;default:idle" json:"last_run_status"` // idle, running, success, failed
	LastRunOutput string     `gorm:"type:text" json:"last_run_output"`
	LastRunAt     *time.Time `json:"last_run_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

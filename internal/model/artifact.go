package model

import "time"

// Artifact represents an uploaded build artifact
type Artifact struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	BuildID       uint      `gorm:"index;not null" json:"build_id"`
	ComponentName string    `gorm:"size:128" json:"component_name"`
	FileName      string    `gorm:"size:256" json:"file_name"`
	FilePath      string    `gorm:"size:512" json:"file_path"`
	FileSize      int64     `json:"file_size"`
	ContentType   string    `gorm:"size:128" json:"content_type"`
	UploadedBy    string    `gorm:"size:64" json:"uploaded_by"` // "jenkins" or username
	Build         *Build    `gorm:"foreignKey:BuildID" json:"build,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

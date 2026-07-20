package model

import "time"

// FAQ represents a frequently asked question entry
type FAQ struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Question  string    `gorm:"size:512;not null" json:"question"`
	Answer    string    `gorm:"type:text" json:"answer"`
	CreatedBy string    `gorm:"size:64" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

package model

// ReleaseComponent represents a component snapshot within a Release
type ReleaseComponent struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	ReleaseID        uint       `gorm:"index;not null" json:"release_id"`
	ComponentID      uint       `gorm:"index;not null" json:"component_id"`
	ComponentName    string     `gorm:"size:128" json:"component_name"`
	ComponentVersion string     `gorm:"size:64" json:"component_version"`
	GitBranch        string     `gorm:"size:128" json:"git_branch"`
	GitCommit        string     `gorm:"size:64" json:"git_commit"`
	ArtifactFile     string     `gorm:"size:256" json:"artifact_file"`
	BuildStatus      string     `gorm:"size:32;default:pending" json:"build_status"`
	Component        *Component `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	Release          *Release   `gorm:"foreignKey:ReleaseID" json:"-"`
}

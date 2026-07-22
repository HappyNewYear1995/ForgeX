package model

import "time"

type BuildStatus string

const (
	BuildStatusPending BuildStatus = "pending"
	BuildStatusRunning BuildStatus = "running"
	BuildStatusSuccess BuildStatus = "success"
	BuildStatusFailed  BuildStatus = "failed"
)

// Build represents a Jenkins build execution record
type Build struct {
	ID                 uint        `gorm:"primaryKey" json:"id"`
	ProductID          uint        `gorm:"index;not null" json:"product_id"`
	ReleaseID          uint        `gorm:"index" json:"release_id"`
	JenkinsBuildNumber int         `json:"jenkins_build_number"`
	ProductVersion     string      `gorm:"size:64" json:"product_version"`
	BuildEnv           string      `gorm:"size:32;default:dev" json:"build_env"`
	Status             BuildStatus `gorm:"size:32;default:pending" json:"status"`
	TriggeredBy        string      `gorm:"size:64" json:"triggered_by"`
	ReleaseNotes       string      `gorm:"type:text" json:"release_notes"`
	ParametersJSON     string      `gorm:"type:text" json:"parameters_json"`
	LogURL             string      `gorm:"size:512" json:"log_url"`
	ScriptRunStatus      string      `gorm:"size:32;default:" json:"script_run_status"` // "", running, success, failed
	ScriptRunOutput      string      `gorm:"type:text" json:"script_run_output"`
	RunScriptsAfterBuild bool        `gorm:"default:false" json:"run_scripts_after_build"`
	CallbackToken        string      `gorm:"size:64;index" json:"callback_token"`
	JenkinsJobName       string      `gorm:"size:256" json:"jenkins_job_name"`
	Product            *Product    `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Release            *Release    `gorm:"foreignKey:ReleaseID" json:"release,omitempty"`
	StartedAt          *time.Time  `json:"started_at"`
	FinishedAt         *time.Time  `json:"finished_at"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

package main

import "time"

const (
	AdminRoleAdmin = "admin"
	AdminRoleUser  = "user"

	AdminStatusActive   = "active"
	AdminStatusDisabled = "disabled"
)

type DataMigration struct {
	Name      string    `gorm:"primaryKey;size:120"`
	CreatedAt time.Time `gorm:"not null"`
}

type Admin struct {
	ID                    uint       `json:"id" gorm:"primaryKey"`
	Username              string     `json:"username" gorm:"uniqueIndex;size:80;not null"`
	PasswordHash          string     `json:"-"`
	Role                  string     `json:"role" gorm:"index;size:20;not null;default:user"`
	Status                string     `json:"status" gorm:"size:20;not null;default:active"`
	AuthVersion           uint       `json:"-" gorm:"not null;default:1"`
	MustChangeCredentials bool       `json:"must_change_credentials" gorm:"not null;default:false"`
	LastLoginAt           *time.Time `json:"last_login_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type PublicFontItem struct {
	ID          uint      `json:"id"`
	FontName    string    `json:"font_name"`
	FirstSeenAt time.Time `json:"first_seen_at"`
}

type SiteSetting struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:120;not null"`
	Subtitle       string    `json:"subtitle" gorm:"size:240"`
	LogoPath       string    `json:"logo_path"`
	LanzouPassword string    `json:"-" gorm:"size:80"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DocumentSource struct {
	ID                     uint       `json:"id" gorm:"primaryKey"`
	URL                    string     `json:"url" gorm:"uniqueIndex;not null"`
	Title                  string     `json:"title" gorm:"size:255"`
	RefreshIntervalMinutes int        `json:"refresh_interval_minutes" gorm:"not null;default:60"`
	Enabled                bool       `json:"enabled" gorm:"not null;default:true;index:idx_source_schedule,priority:1"`
	LastParsedAt           *time.Time `json:"last_parsed_at"`
	NextRunAt              *time.Time `json:"next_run_at" gorm:"index:idx_source_schedule,priority:2"`
	LastError              string     `json:"last_error"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type FontItem struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	SourceID         uint      `json:"source_id" gorm:"uniqueIndex:idx_font_unique;index:idx_font_document_order,priority:1;not null"`
	FontName         string    `json:"font_name" gorm:"uniqueIndex:idx_font_unique;not null"`
	DownloadURL      string    `json:"download_url" gorm:"uniqueIndex:idx_font_unique;not null"`
	AccessCode       string    `json:"access_code" gorm:"size:80"`
	DocumentPosition int       `json:"document_position" gorm:"index:idx_font_document_order,priority:2;not null;default:0"`
	FirstSeenAt      time.Time `json:"first_seen_at" gorm:"index:idx_font_seen"`
	LastSeenAt       time.Time `json:"last_seen_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ParseRun struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	SourceID   uint      `json:"source_id" gorm:"index;not null"`
	Status     string    `json:"status" gorm:"size:20;not null"`
	TotalFound int       `json:"total_found"`
	Inserted   int       `json:"inserted"`
	Error      string    `json:"error"`
	StartedAt  time.Time `json:"started_at" gorm:"index:idx_parse_started"`
	FinishedAt time.Time `json:"finished_at"`
	CreatedAt  time.Time `json:"created_at"`
}

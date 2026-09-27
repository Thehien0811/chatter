package domain

import "time"

// User represents the users table in the README schema.
type User struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	DisplayName   string     `json:"display_name"`
	Email         string     `json:"email,omitempty"`
	PasswordHash  string     `json:"-"`
	AvatarMediaID string     `json:"avatar_media_id,omitempty"`
	StatusMessage string     `json:"status_message,omitempty"`
	Presence      string     `json:"presence"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

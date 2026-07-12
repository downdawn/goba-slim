// Package user 提供用户领域模型与应用能力。
package user

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusArchived Status = "archived"
)

func (s Status) Valid() bool {
	return s == StatusActive || s == StatusDisabled || s == StatusArchived
}

type User struct {
	ID                    uuid.UUID
	Username              string
	PasswordHash          string
	DisplayName           string
	Email                 *string
	AvatarURL             *string
	Status                Status
	IsSuperuser           bool
	AllowMultipleSessions bool
	SessionVersion        int64
	PasswordChangedAt     time.Time
	LastLoginAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ArchivedAt            *time.Time
}

type CreateInput struct {
	Username              string
	Password              string
	DisplayName           string
	Email                 string
	AvatarURL             string
	IsSuperuser           bool
	AllowMultipleSessions bool
}

type UpdateProfileInput struct {
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
}

type ListFilter struct {
	Username string
	Status   Status
	Page     int32
	Size     int32
}

type Page struct {
	Items []User
	Total int64
	Page  int32
	Size  int32
}

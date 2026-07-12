package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(context.Context, User) (User, error)
	GetByID(context.Context, uuid.UUID) (User, error)
	GetByUsername(context.Context, string) (User, error)
	List(context.Context, ListFilter, int32, int32) ([]User, int64, error)
	UpdateProfile(context.Context, uuid.UUID, UpdateProfileInput, time.Time) (User, error)
	SetStatus(context.Context, uuid.UUID, Status, time.Time) (User, error)
	SetSuperuser(context.Context, uuid.UUID, bool, time.Time) (User, error)
	SetMultipleSessions(context.Context, uuid.UUID, bool, time.Time) (User, error)
	UpdatePassword(context.Context, uuid.UUID, string, time.Time) (User, error)
	UpdateLastLogin(context.Context, uuid.UUID, time.Time) error
	LockSuperuserChanges(context.Context) error
	CountActiveSuperusers(context.Context) (int64, error)
}

type UnitOfWork interface {
	WithinTransaction(context.Context, func(Repository) error) error
}

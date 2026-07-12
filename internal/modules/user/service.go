package user

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/downdawn/goba-slim/internal/shared/clock"
	"github.com/downdawn/goba-slim/internal/shared/id"
	"github.com/downdawn/goba-slim/internal/shared/pagination"
	"github.com/google/uuid"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

type Service struct {
	repository Repository
	unitOfWork UnitOfWork
	passwords  PasswordManager
	dummyHash  string
	clock      clock.Clock
	ids        id.Generator
}

func NewService(repository Repository, unitOfWork UnitOfWork, passwords PasswordManager, businessClock clock.Clock, ids id.Generator) (*Service, error) {
	if repository == nil || unitOfWork == nil || passwords == nil || businessClock == nil || ids == nil {
		return nil, fmt.Errorf("用户服务依赖不能为空")
	}
	dummyHash, err := passwords.Hash("TimingDefensePassword9")
	if err != nil {
		return nil, fmt.Errorf("构造密码时序防护摘要: %w", err)
	}
	return &Service{repository: repository, unitOfWork: unitOfWork, passwords: passwords, dummyHash: dummyHash, clock: businessClock, ids: ids}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (User, error) {
	input, err := normalizeCreateInput(input)
	if err != nil {
		return User{}, err
	}
	if err := s.passwords.Validate(input.Password); err != nil {
		return User{}, ErrInvalidPassword
	}
	hash, err := s.passwords.Hash(input.Password)
	if err != nil {
		return User{}, fmt.Errorf("计算密码摘要: %w", err)
	}
	identifier, err := s.ids.New()
	if err != nil {
		return User{}, fmt.Errorf("生成用户 ID: %w", err)
	}
	now := s.clock.Now().UTC()
	created, err := s.repository.Create(ctx, User{
		ID: identifier, Username: input.Username, PasswordHash: hash, DisplayName: input.DisplayName,
		Email: optionalString(input.Email), AvatarURL: optionalString(input.AvatarURL), Status: StatusActive,
		IsSuperuser: input.IsSuperuser, AllowMultipleSessions: input.AllowMultipleSessions,
		SessionVersion: 1, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return User{}, err
	}
	return created, nil
}

func (s *Service) CreateAdmin(ctx context.Context, input CreateInput) (User, error) {
	input.IsSuperuser = true
	return s.Create(ctx, input)
}

func (s *Service) GetByID(ctx context.Context, userID uuid.UUID) (User, error) {
	return s.repository.GetByID(ctx, userID)
}

func (s *Service) GetByUsername(ctx context.Context, username string) (User, error) {
	return s.repository.GetByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
}

func (s *Service) VerifyCredentials(ctx context.Context, username, password string) (User, error) {
	current, err := s.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			_, _ = s.passwords.Verify(password, s.dummyHash)
		}
		return User{}, err
	}
	matched, err := s.passwords.Verify(password, current.PasswordHash)
	if err != nil {
		return User{}, fmt.Errorf("校验用户密码: %w", err)
	}
	if !matched {
		return User{}, ErrPasswordMismatch
	}
	return current, nil
}

func (s *Service) RecordLogin(ctx context.Context, userID uuid.UUID) error {
	return s.repository.UpdateLastLogin(ctx, userID, s.clock.Now().UTC())
}

func (s *Service) List(ctx context.Context, filter ListFilter) (Page, error) {
	params := pagination.Params{Page: filter.Page, Size: filter.Size}.Normalize()
	filter.Username = strings.TrimSpace(filter.Username)
	if filter.Status != "" && !filter.Status.Valid() {
		return Page{}, ErrInvalidInput
	}
	items, total, err := s.repository.List(ctx, filter, params.Size, params.Offset())
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Page: params.Page, Size: params.Size}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (User, error) {
	normalized, err := normalizeProfile(input)
	if err != nil {
		return User{}, err
	}
	return s.repository.UpdateProfile(ctx, userID, normalized, s.clock.Now().UTC())
}

func (s *Service) SetStatus(ctx context.Context, userID uuid.UUID, status Status) (User, error) {
	if !status.Valid() {
		return User{}, ErrInvalidInput
	}
	return s.protectLastSuperuser(ctx, userID, func(repository Repository, current User) (User, error) {
		if current.Status == status {
			return current, nil
		}
		return repository.SetStatus(ctx, userID, status, s.clock.Now().UTC())
	}, status != StatusActive)
}

func (s *Service) SetSuperuser(ctx context.Context, userID uuid.UUID, enabled bool) (User, error) {
	return s.protectLastSuperuser(ctx, userID, func(repository Repository, current User) (User, error) {
		if current.IsSuperuser == enabled {
			return current, nil
		}
		return repository.SetSuperuser(ctx, userID, enabled, s.clock.Now().UTC())
	}, !enabled)
}

func (s *Service) SetMultipleSessions(ctx context.Context, userID uuid.UUID, enabled bool) (User, error) {
	return s.repository.SetMultipleSessions(ctx, userID, enabled, s.clock.Now().UTC())
}

func (s *Service) ResetPassword(ctx context.Context, userID uuid.UUID, password string) (User, error) {
	if err := s.passwords.Validate(password); err != nil {
		return User{}, ErrInvalidPassword
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return User{}, fmt.Errorf("计算密码摘要: %w", err)
	}
	return s.repository.UpdatePassword(ctx, userID, hash, s.clock.Now().UTC())
}

func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (User, error) {
	current, err := s.repository.GetByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	matched, err := s.passwords.Verify(oldPassword, current.PasswordHash)
	if err != nil {
		return User{}, fmt.Errorf("校验原密码: %w", err)
	}
	if !matched {
		return User{}, ErrPasswordMismatch
	}
	return s.ResetPassword(ctx, userID, newPassword)
}

func (s *Service) Archive(ctx context.Context, userID uuid.UUID) (User, error) {
	return s.SetStatus(ctx, userID, StatusArchived)
}

func (s *Service) protectLastSuperuser(ctx context.Context, userID uuid.UUID, change func(Repository, User) (User, error), mayRemove bool) (User, error) {
	var result User
	err := s.unitOfWork.WithinTransaction(ctx, func(repository Repository) error {
		if err := repository.LockSuperuserChanges(ctx); err != nil {
			return err
		}
		current, err := repository.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if mayRemove && current.IsSuperuser && current.Status == StatusActive {
			count, countErr := repository.CountActiveSuperusers(ctx)
			if countErr != nil {
				return countErr
			}
			if count <= 1 {
				return ErrLastSuperuser
			}
		}
		result, err = change(repository, current)
		return err
	})
	if err != nil {
		return User{}, err
	}
	return result, nil
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	profile, err := normalizeProfile(UpdateProfileInput{Username: input.Username, DisplayName: input.DisplayName, Email: input.Email, AvatarURL: input.AvatarURL})
	if err != nil {
		return CreateInput{}, err
	}
	input.Username, input.DisplayName, input.Email, input.AvatarURL = profile.Username, profile.DisplayName, profile.Email, profile.AvatarURL
	return input, nil
}

func normalizeProfile(input UpdateProfileInput) (UpdateProfileInput, error) {
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	if !usernamePattern.MatchString(input.Username) {
		return UpdateProfileInput{}, fmt.Errorf("%w: username", ErrInvalidInput)
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}
	if len([]rune(input.DisplayName)) > 64 {
		return UpdateProfileInput{}, fmt.Errorf("%w: display_name", ErrInvalidInput)
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Email != "" {
		parsed, err := mail.ParseAddress(input.Email)
		if err != nil || parsed.Address != input.Email || len(input.Email) > 254 {
			return UpdateProfileInput{}, fmt.Errorf("%w: email", ErrInvalidInput)
		}
	}
	input.AvatarURL = strings.TrimSpace(input.AvatarURL)
	if input.AvatarURL != "" {
		parsed, err := url.ParseRequestURI(input.AvatarURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return UpdateProfileInput{}, fmt.Errorf("%w: avatar_url", ErrInvalidInput)
		}
	}
	return input, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

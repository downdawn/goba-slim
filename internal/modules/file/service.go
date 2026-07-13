package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/downdawn/goba-slim/internal/shared/id"
	"github.com/google/uuid"
)

const sniffBytes = 512

type Object struct {
	Content      io.ReadSeekCloser
	Size         int64
	ModifiedTime time.Time
}

type ObjectStore interface {
	Put(context.Context, ObjectKey, io.Reader, int64) (int64, error)
	Open(context.Context, ObjectKey) (Object, error)
	Delete(context.Context, ObjectKey) error
}

type Limits struct {
	ImageMaxBytes int64
	VideoEnabled  bool
	VideoMaxBytes int64
}

type Service struct {
	store ObjectStore
	ids   id.Generator
	limit Limits
}

func NewService(store ObjectStore, ids id.Generator, limits Limits) (*Service, error) {
	if store == nil || ids == nil || limits.ImageMaxBytes < sniffBytes || limits.VideoMaxBytes < sniffBytes {
		return nil, fmt.Errorf("文件服务依赖或限制无效")
	}
	return &Service{store: store, ids: ids, limit: limits}, nil
}

func (s *Service) Upload(ctx context.Context, ownerID uuid.UUID, content io.Reader) (Uploaded, error) {
	if ownerID == uuid.Nil || content == nil {
		return Uploaded{}, ErrInvalidFile
	}
	header := make([]byte, sniffBytes)
	read, err := io.ReadFull(content, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Uploaded{}, fmt.Errorf("%w: 读取文件头: %w", ErrUnavailable, err)
	}
	if read == 0 {
		return Uploaded{}, ErrInvalidFile
	}
	header = header[:read]
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(header), ";")[0]))
	ext, maxBytes, ok := s.allowedType(mediaType)
	if !ok {
		return Uploaded{}, ErrTypeNotAllowed
	}
	objectID, err := s.ids.New()
	if err != nil {
		return Uploaded{}, fmt.Errorf("%w: 生成对象标识: %w", ErrUnavailable, err)
	}
	key := ObjectKey{OwnerID: ownerID, ObjectID: objectID, Ext: ext}
	size, err := s.store.Put(ctx, key, io.MultiReader(bytes.NewReader(header), content), maxBytes)
	if err != nil {
		return Uploaded{}, err
	}
	return Uploaded{Key: key, ContentType: mediaType, Size: size}, nil
}

func (s *Service) Open(ctx context.Context, value string) (Uploaded, Object, error) {
	key, err := ParseObjectKey(value)
	if err != nil {
		return Uploaded{}, Object{}, err
	}
	mediaType, _, ok := s.allowedExtension(key.Ext)
	if !ok {
		return Uploaded{}, Object{}, ErrInvalidKey
	}
	object, err := s.store.Open(ctx, key)
	if err != nil {
		return Uploaded{}, Object{}, err
	}
	return Uploaded{Key: key, ContentType: mediaType, Size: object.Size}, object, nil
}

func (s *Service) Delete(ctx context.Context, actorID uuid.UUID, superuser bool, value string) error {
	key, err := ParseObjectKey(value)
	if err != nil {
		return err
	}
	if actorID == uuid.Nil || (!superuser && actorID != key.OwnerID) {
		return ErrForbidden
	}
	return s.store.Delete(ctx, key)
}

func (s *Service) allowedType(mediaType string) (string, int64, bool) {
	switch mediaType {
	case "image/jpeg":
		return "jpg", s.limit.ImageMaxBytes, true
	case "image/png":
		return "png", s.limit.ImageMaxBytes, true
	case "image/gif":
		return "gif", s.limit.ImageMaxBytes, true
	case "image/webp":
		return "webp", s.limit.ImageMaxBytes, true
	case "video/mp4":
		return "mp4", s.limit.VideoMaxBytes, s.limit.VideoEnabled
	case "video/webm":
		return "webm", s.limit.VideoMaxBytes, s.limit.VideoEnabled
	default:
		return "", 0, false
	}
}

func (s *Service) allowedExtension(ext string) (string, int64, bool) {
	switch ext {
	case "jpg":
		return "image/jpeg", s.limit.ImageMaxBytes, true
	case "png":
		return "image/png", s.limit.ImageMaxBytes, true
	case "gif":
		return "image/gif", s.limit.ImageMaxBytes, true
	case "webp":
		return "image/webp", s.limit.ImageMaxBytes, true
	case "mp4":
		return "video/mp4", s.limit.VideoMaxBytes, true
	case "webm":
		return "video/webm", s.limit.VideoMaxBytes, true
	default:
		return "", 0, false
	}
}

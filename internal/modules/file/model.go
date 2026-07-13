// Package file 提供公开业务素材的安全上传与存储能力。
package file

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

type ObjectKey struct {
	OwnerID  uuid.UUID
	ObjectID uuid.UUID
	Ext      string
}

func (k ObjectKey) String() string {
	return k.OwnerID.String() + "/" + k.ObjectID.String() + "." + k.Ext
}

func ParseObjectKey(value string) (ObjectKey, error) {
	if value == "" || strings.Contains(value, "\\") || path.Clean(value) != value {
		return ObjectKey{}, ErrInvalidKey
	}
	owner, name, ok := strings.Cut(value, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return ObjectKey{}, ErrInvalidKey
	}
	object, ext, ok := strings.Cut(name, ".")
	if !ok || object == "" || ext == "" || strings.Contains(ext, ".") || ext != strings.ToLower(ext) {
		return ObjectKey{}, ErrInvalidKey
	}
	ownerID, err := uuid.Parse(owner)
	if err != nil || ownerID.String() != owner {
		return ObjectKey{}, ErrInvalidKey
	}
	objectID, err := uuid.Parse(object)
	if err != nil || objectID.String() != object || !allowedExtension(ext) {
		return ObjectKey{}, ErrInvalidKey
	}
	return ObjectKey{OwnerID: ownerID, ObjectID: objectID, Ext: ext}, nil
}

type Uploaded struct {
	Key         ObjectKey
	ContentType string
	Size        int64
}

func (u Uploaded) URL() string { return "/files/" + u.Key.String() }

func allowedExtension(ext string) bool {
	switch ext {
	case "jpg", "png", "gif", "webp", "mp4", "webm":
		return true
	default:
		return false
	}
}

func (k ObjectKey) Validate() error {
	parsed, err := ParseObjectKey(k.String())
	if err != nil || parsed != k {
		return fmt.Errorf("%w: 对象 Key 无效", ErrInvalidKey)
	}
	return nil
}

// Package id 提供业务标识生成能力。
package id

import "github.com/google/uuid"

type Generator interface {
	New() (uuid.UUID, error)
}

type UUIDv7 struct{}

func (UUIDv7) New() (uuid.UUID, error) { return uuid.NewV7() }

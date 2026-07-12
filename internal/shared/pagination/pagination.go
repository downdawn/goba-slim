// Package pagination 定义跨模块稳定的分页输入。
package pagination

const (
	DefaultPageSize int32 = 20
	MaxPageSize     int32 = 100
)

type Params struct {
	Page int32
	Size int32
}

func (p Params) Normalize() Params {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Size < 1 {
		p.Size = DefaultPageSize
	}
	if p.Size > MaxPageSize {
		p.Size = MaxPageSize
	}
	return p
}

func (p Params) Offset() int32 {
	p = p.Normalize()
	return (p.Page - 1) * p.Size
}

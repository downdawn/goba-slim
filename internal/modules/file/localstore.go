package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"
)

type LocalStore struct {
	path string
	mu   sync.RWMutex
	root *os.Root
}

func NewLocalStore(storagePath string) (*LocalStore, error) {
	if storagePath == "" {
		return nil, fmt.Errorf("文件存储目录不能为空")
	}
	return &LocalStore{path: storagePath}, nil
}

func (s *LocalStore) Start(_ context.Context) error {
	if err := os.MkdirAll(s.path, 0o750); err != nil {
		return fmt.Errorf("%w: 创建存储目录: %w", ErrUnavailable, err)
	}
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return fmt.Errorf("%w: 打开存储目录: %w", ErrUnavailable, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root != nil {
		_ = root.Close()
		return nil
	}
	s.root = root
	return nil
}

func (s *LocalStore) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil
	}
	err := s.root.Close()
	s.root = nil
	return err
}

func (s *LocalStore) Health(_ context.Context) error {
	root, err := s.activeRoot()
	if err != nil {
		return err
	}
	if _, err := root.Stat("."); err != nil {
		return fmt.Errorf("%w: 检查存储目录: %w", ErrUnavailable, err)
	}
	return nil
}

func (s *LocalStore) Put(ctx context.Context, key ObjectKey, content io.Reader, maxBytes int64) (int64, error) {
	if err := key.Validate(); err != nil || content == nil || maxBytes < 1 {
		return 0, ErrInvalidKey
	}
	root, err := s.activeRoot()
	if err != nil {
		return 0, err
	}
	owner := key.OwnerID.String()
	if err := root.MkdirAll(owner, 0o750); err != nil {
		return 0, fmt.Errorf("%w: 创建对象目录: %w", ErrUnavailable, err)
	}
	temporary := path.Join(owner, "."+key.ObjectID.String()+".tmp")
	destination := key.String()
	output, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("%w: 创建临时对象: %w", ErrUnavailable, err)
	}
	committed := false
	defer func() {
		_ = output.Close()
		if !committed {
			_ = root.Remove(temporary)
		}
	}()

	written, copyErr := io.CopyBuffer(output, io.LimitReader(contextReader{ctx: ctx, reader: content}, maxBytes+1), make([]byte, 32<<10))
	if copyErr != nil {
		return 0, fmt.Errorf("%w: 写入临时对象: %w", ErrUnavailable, copyErr)
	}
	if written > maxBytes {
		return 0, ErrTooLarge
	}
	if err := output.Sync(); err != nil {
		return 0, fmt.Errorf("%w: 同步临时对象: %w", ErrUnavailable, err)
	}
	if err := output.Close(); err != nil {
		return 0, fmt.Errorf("%w: 关闭临时对象: %w", ErrUnavailable, err)
	}
	if err := root.Rename(temporary, destination); err != nil {
		return 0, fmt.Errorf("%w: 提交对象: %w", ErrUnavailable, err)
	}
	committed = true
	return written, nil
}

func (s *LocalStore) Open(_ context.Context, key ObjectKey) (Object, error) {
	if err := key.Validate(); err != nil {
		return Object{}, err
	}
	root, err := s.activeRoot()
	if err != nil {
		return Object{}, err
	}
	opened, err := root.Open(key.String())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Object{}, ErrNotFound
		}
		return Object{}, fmt.Errorf("%w: 打开对象: %w", ErrUnavailable, err)
	}
	info, err := opened.Stat()
	if err != nil {
		_ = opened.Close()
		return Object{}, fmt.Errorf("%w: 读取对象信息: %w", ErrUnavailable, err)
	}
	if !info.Mode().IsRegular() {
		_ = opened.Close()
		return Object{}, ErrNotFound
	}
	return Object{Content: opened, Size: info.Size(), ModifiedTime: info.ModTime()}, nil
}

func (s *LocalStore) Delete(_ context.Context, key ObjectKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	root, err := s.activeRoot()
	if err != nil {
		return err
	}
	if err := root.Remove(key.String()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("%w: 删除对象: %w", ErrUnavailable, err)
	}
	return nil
}

func (s *LocalStore) activeRoot() (*os.Root, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.root == nil {
		return nil, ErrUnavailable
	}
	return s.root, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

package aggregate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// PayloadSpiller 大包 payload 的磁盘溢出存储。
// 用于时间轮分级缓存：大包（占内存主体的少数包）落盘，时间轮只保留元数据。
type PayloadSpiller interface {
	// Put 写入 payload，返回可在后续 Get/Delete 中使用的 key。
	Put(payload []byte) (key string, err error)
	// Get 按 key 读回 payload。
	Get(key string) ([]byte, error)
	// Delete 删除 key 对应的数据（幂等）。
	Delete(key string) error
	// Close 关闭存储。
	Close() error
}

// FilePayloadSpiller 目录 + 文件实现的 PayloadSpiller。
// 打开时清空目录，清理上一次运行遗留的 spill 数据（spill 生命周期不超过进程生命周期）。
type FilePayloadSpiller struct {
	dir string
	seq atomic.Int64
}

func NewFilePayloadSpiller(dir string) (*FilePayloadSpiller, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("clear spill dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create spill dir: %w", err)
	}

	return &FilePayloadSpiller{dir: dir}, nil
}

func (f *FilePayloadSpiller) Put(payload []byte) (string, error) {
	if f == nil {
		return "", errors.New("file payload spiller is nil")
	}

	key := fmt.Sprintf("%x_%d", time.Now().UnixNano(), f.seq.Add(1))
	path := filepath.Join(f.dir, key)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return "", fmt.Errorf("write spill payload: %w", err)
	}

	return key, nil
}

func (f *FilePayloadSpiller) Get(key string) ([]byte, error) {
	if f == nil {
		return nil, errors.New("file payload spiller is nil")
	}
	if !validSpillKey(key) {
		return nil, fmt.Errorf("invalid spill key %q", key)
	}

	payload, err := os.ReadFile(filepath.Join(f.dir, key))
	if err != nil {
		return nil, fmt.Errorf("read spill payload: %w", err)
	}

	return payload, nil
}

func (f *FilePayloadSpiller) Delete(key string) error {
	if f == nil || !validSpillKey(key) {
		return nil
	}

	if err := os.Remove(filepath.Join(f.dir, key)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete spill payload: %w", err)
	}

	return nil
}

func (f *FilePayloadSpiller) Close() error {
	return nil
}

// validSpillKey 限制 key 的字符集，防止路径穿越。
func validSpillKey(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}

	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c == '_':
		default:
			return false
		}
	}

	return true
}

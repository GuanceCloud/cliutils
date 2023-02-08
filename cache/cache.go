// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package cache wraps wal to handle local disk cache.
package cache

import (
	"errors"
	"os"
	"sync"

	"github.com/tidwall/wal"
)

type Cache struct {
	l *wal.Log

	lock *sync.Mutex
	path string

	totalMsg uint64

	closed bool

	cap, size uint64
}

func New(path string, capacity uint64) (*Cache, error) {
	l, err := wal.Open(path, &wal.Options{NoCopy: true, LogFormat: wal.JSON})
	if err != nil {
		return nil, err
	}

	return &Cache{
		l:    l,
		path: path,
		lock: &sync.Mutex{},
		cap:  capacity,
	}, nil
}

var (
	ErrExceedCapacity = errors.New("exceed cache max capacity")
	ErrCacheClosed    = errors.New("cache closed")
)

func (c *Cache) doPut(data []byte) error {
	if c.closed {
		if err := c.reopen(); err != nil {
			return err
		}
	}

	if c.cap > 0 && c.size+uint64(len(data)) > c.cap {
		return ErrExceedCapacity
	}

	idx, err := c.l.LastIndex()
	if err != nil {
		return err
	}

	if err := c.l.Write(idx+1, data); err != nil {
		return err
	}

	c.size += uint64(len(data))
	c.totalMsg++
	return nil
}

func (c *Cache) Put(data []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.doPut(data)
}

type Fn func([]byte) error

func (c *Cache) Get(fn Fn) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.closed {
		return nil
	}

	idx, err := c.l.FirstIndex()
	if err != nil {
		return err
	}

	data, err := c.l.Read(idx)
	if err != nil {
		return err
	}

	if err := fn(data); err != nil {
		return err
	}

	if c.lastEntry() {
		if err := c.doClear(); err != nil {
			return err
		}

		if err := c.reopen(); err != nil {
			return err
		}
	} else {
		if err := c.l.TruncateFront(idx + 1); err != nil {
			return err
		}
	}
	c.totalMsg--

	return nil
}

func (c *Cache) doClear() error {
	if err := c.l.Close(); err != nil {
		return err
	}

	if err := os.RemoveAll(c.path); err != nil {
		return err
	}

	c.size = 0
	c.closed = true
	return nil
}

func (c *Cache) Clear() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.doClear()
}

func (c *Cache) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.l.Close()
}

func (c *Cache) lastEntry() bool {
	first, err := c.l.FirstIndex()
	if err != nil {
		return false
	}

	last, err := c.l.LastIndex()
	if err != nil {
		return false
	}

	return first == last
}

func (c *Cache) reopen() error {
	l, err := wal.Open(c.path, nil)
	if err != nil {
		return err
	}
	c.l = l
	c.closed = false
	return nil
}

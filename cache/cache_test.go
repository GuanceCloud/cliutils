// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package cache

import (
	// "os"

	"strings"
	"sync"
	"testing"
	"time"
)

func TestPut(t *testing.T) {
	cases := []struct {
		name string
		data []string
	}{
		{
			name: "basic",
			data: []string{
				"hello world",
				"hello 1024",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache, err := New("abc", 0)
			if err != nil {
				t.Error(err)
				return
			}

			defer func() {
				if err := cache.Close(); err != nil {
					t.Error(err)
				}
			}()

			for _, x := range tc.data {
				if err := cache.Put([]byte(x)); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestConcurrentPutGet(t *testing.T) {
	cases := []struct {
		name string
		data []string
	}{
		{
			name: "short",
			data: []string{
				"hello world",
				"hello 1024",
				"hello 2048",
				"hello 3048",
				"hello 4048",
				"hello 5048",
				"hello 6048",
			},
		},

		{
			name: "large",
			data: []string{
				strings.Repeat("hello world", 1000), strings.Repeat("hello 1024", 1000), strings.Repeat("hello 2048", 1000),
				strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000), strings.Repeat("hello 5048", 1000),
				strings.Repeat("hello 6048", 1000), strings.Repeat("hello world", 1000), strings.Repeat("hello world", 1000),
				strings.Repeat("hello world", 1000), strings.Repeat("hello world", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello world", 1000), strings.Repeat("hello 1024", 1000), strings.Repeat("hello 2048", 1000),
				strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000), strings.Repeat("hello 5048", 1000),
				strings.Repeat("hello 6048", 1000), strings.Repeat("hello world", 1000), strings.Repeat("hello world", 1000),
				strings.Repeat("hello world", 1000), strings.Repeat("hello world", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 6048", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000), strings.Repeat("hello 6048", 1000),
			},
		},

		{
			name: "mixed",
			data: []string{
				strings.Repeat("hello world", 1000), "hello world", "hello 1024",
				"hello 5048", "hello 6048", strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), "hello 2048",
				"hello 3048", "hello 4048", strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000),
				strings.Repeat("hello world", 1000), "hello world", "hello 1024",
				strings.Repeat("hello world", 1000), "hello world", "hello 1024",
				strings.Repeat("hello world", 1000), "hello world", "hello 1024",
				strings.Repeat("hello world", 1000), "hello world", "hello 1024",
				strings.Repeat("hello world", 1000), "hello world", "hello 1024",
				"hello 5048", "hello 6048", strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), "hello 2048",
				"hello 3048", "hello 4048", strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000),
				"hello 5048", "hello 6048", strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), "hello 2048",
				"hello 3048", "hello 4048", strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000),
				"hello 5048", "hello 6048", strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), "hello 2048",
				"hello 3048", "hello 4048", strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000),
				"hello 5048", "hello 6048", strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), "hello 2048",
				"hello 3048", "hello 4048", strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000),
				"hello 5048", "hello 6048", strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000), strings.Repeat("hello 3048", 1000), "hello 2048",
				"hello 3048", "hello 4048", strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000), strings.Repeat("hello 6048", 1000),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var wg sync.WaitGroup

			cache, err := New("abc", 0)
			if err != nil {
				t.Error(err)
				return
			}

			wg.Add(len(tc.data))
			for _, x := range tc.data { // multiple writer
				go func(str string) {
					defer wg.Done()
					i := 0
					for {
						i++
						if err := cache.Put([]byte(str)); err != nil {
							t.Error(err)
						}

						if i > 100 {
							t.Logf("Put done.")
							return
						}
					}
				}(x)
			}

			time.Sleep(time.Second)

			wg.Add(4)
			for i := 0; i < 4; i++ {
				go func() { // multiple reader
					defer wg.Done()
					tick := time.NewTicker(time.Millisecond * 200)
					defer tick.Stop()
					for range tick.C {
						start := time.Now()
						if err := cache.Get(func(data []byte) error {
							t.Logf("get data(%d bytes), cost: %s", len(data), time.Since(start))
							return nil
						}); err != nil {
							t.Logf("Get: %s", err)
							return
						}
					}
				}()
			}

			wg.Wait()

			if err := cache.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestConcurrentPut(t *testing.T) {
	cases := []struct {
		name string
		data []string
	}{
		{
			name: "short",
			data: []string{
				"hello world",
				"hello 1024",
				"hello 2048",
				"hello 3048",
				"hello 4048",
				"hello 5048",
				"hello 6048",
			},
		},

		{
			name: "large",
			data: []string{
				strings.Repeat("hello world", 1000),
				strings.Repeat("hello 1024", 1000),
				strings.Repeat("hello 2048", 1000),
				strings.Repeat("hello 3048", 1000),
				strings.Repeat("hello 4048", 1000),
				strings.Repeat("hello 5048", 1000),
				strings.Repeat("hello 6048", 1000),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var wg sync.WaitGroup

			cache, err := New("abc", 0)
			if err != nil {
				t.Error(err)
				return
			}

			defer func() {
				if err := cache.Close(); err != nil {
					t.Error(err)
				}
			}()

			wg.Add(len(tc.data))
			for _, x := range tc.data {
				go func(str string) {
					defer wg.Done()
					i := 0
					for {
						i++
						if err := cache.Put([]byte(str)); err != nil {
							t.Error(err)
						}

						if i > 100 {
							return
						}
					}
				}(x)
			}

			wg.Wait()
		})
	}
}

func TestGet(t *testing.T) {
	cases := []struct {
		name string
		data []string
	}{
		{
			name: "basic",
			data: []string{
				"hello world",
				"hello 1024",
				"hello 2048",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache, err := New("abc", 0)
			if err != nil {
				t.Error(err)
				return
			}

			defer func() {
				if err := cache.Close(); err != nil {
					t.Error(err)
				}
			}()

			for _, x := range tc.data {
				if err := cache.Put([]byte(x)); err != nil {
					t.Error(err)
				}
			}

			for i := 0; i < len(tc.data)-1; i++ {
				if err := cache.Get(func(data []byte) error {
					t.Logf("get: %s", string(data))
					return nil
				}); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncEnv(t *testing.T) {
	cases := []struct {
		name   string
		envs   map[string]string
		expect *DiskCache
	}{
		{
			name: "env-diskcache-max-data-size",
			envs: map[string]string{
				"ENV_DISKCACHE_MAX_DATA_SIZE": "123",
			},
			expect: func() *DiskCache {
				c := defaultInstance()
				c.maxDataSize = int32(123)
				return c
			}(),
		},

		{
			name: "env-bad-capacity",
			envs: map[string]string{
				"ENV_DISKCACHE_MAX_DATA_SIZE": "123",
				"ENV_DISKCACHE_BATCH_SIZE":    "234",
				"ENV_DISKCACHE_CAPACITY":      "1.2",
			},
			expect: func() *DiskCache {
				c := defaultInstance()
				c.maxDataSize = int32(123)
				c.batchSize = int64(234)
				c.capacity = 0
				return c
			}(),
		},

		{
			name: "env-all",
			envs: map[string]string{
				"ENV_DISKCACHE_MAX_DATA_SIZE": "123",
				"ENV_DISKCACHE_BATCH_SIZE":    "234",
				"ENV_DISKCACHE_CAPACITY":      "1234567890",
				"ENV_DISKCACHE_NO_SYNC":       "foo-bar",
				"ENV_DISKCACHE_NO_POS":        "on",
				"ENV_DISKCACHE_NO_LOCK":       "on",
			},
			expect: func() *DiskCache {
				c := defaultInstance()
				c.maxDataSize = int32(123)
				c.batchSize = int64(234)
				c.capacity = int64(1234567890)
				c.noSync = true
				c.noPos = true
				c.noLock = true
				return c
			}(),
		},
	}
	os.Clearenv()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				if err := os.Setenv(k, v); err != nil {
					t.Error(err)
				}
			}

			c := defaultInstance()
			c.syncEnv()
			assert.Equal(t, tc.expect.String(), c.String())

			t.Cleanup(func() {
				for k := range tc.envs {
					if err := os.Unsetenv(k); err != nil {
						t.Error(err)
					}
				}
			})
		})
	}
}

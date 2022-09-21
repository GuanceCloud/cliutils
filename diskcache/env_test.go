package diskcache

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncEnv(t *testing.T) {
	cases := []struct {
		name   string
		envs   map[string]string
		expect *Option
	}{
		{
			name: "env_diskcache_max_data_size",
			envs: map[string]string{
				"ENV_DISKCACHE_MAX_DATA_SIZE": "123",
			},
			expect: func() *Option {
				opt := defaultOpt()
				opt.MaxDataSize = int64(123)
				return opt
			}(),
		},

		{
			name: "env_bad_capacity",
			envs: map[string]string{
				"ENV_DISKCACHE_MAX_DATA_SIZE": "123",
				"ENV_DISKCACHE_BATCH_SIZE":    "234",
				"ENV_DISKCACHE_CAPACITY":      "1.2",
			},
			expect: func() *Option {
				opt := defaultOpt()
				opt.MaxDataSize = int64(123)
				opt.BatchSize = int64(234)
				opt.Capacity = 0
				return opt
			}(),
		},

		{
			name: "env_all",
			envs: map[string]string{
				"ENV_DISKCACHE_MAX_DATA_SIZE": "123",
				"ENV_DISKCACHE_BATCH_SIZE":    "234",
				"ENV_DISKCACHE_CAPACITY":      "1234567890",
				"ENV_DISKCACHE_NO_SYNC":       "foo-bar",
			},
			expect: func() *Option {
				opt := defaultOpt()
				opt.MaxDataSize = int64(123)
				opt.BatchSize = int64(234)
				opt.Capacity = int64(1234567890)
				opt.NoSync = true
				return opt
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

			opt := defaultOpt()
			opt.syncEnv()
			assert.Equal(t, fmt.Sprintf("%v", tc.expect), fmt.Sprintf("%v", opt))
			t.Logf("opt: %+#v", tc.expect)
		})
	}
}

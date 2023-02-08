// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadEnvs(t *testing.T) {
	cases := []struct {
		name   string
		envs   map[string]string
		expect *Encoder
	}{
		{
			name: `set-pb-encoding`,
			envs: map[string]string{EnvDefaultEncoding: "protobuf"},
			expect: func() *Encoder {
				x := GetEncoder()
				for _, opt := range []EncoderOption{WithEncEncoding(Protobuf)} {
					opt(x)
				}
				return x
			}(),
		},

		{
			name: `set-lp-encoding`,
			envs: map[string]string{EnvDefaultEncoding: "lineprotocol"},
			expect: func() *Encoder {
				x := GetEncoder()
				for _, opt := range []EncoderOption{WithEncEncoding(LineProtocol)} {
					opt(x)
				}
				return x
			}(),
		},

		{
			name: `set-lp-encoding-2`,
			envs: map[string]string{EnvDefaultEncoding: "lineproto"},
			expect: func() *Encoder {
				x := GetEncoder()
				for _, opt := range []EncoderOption{WithEncEncoding(LineProtocol)} {
					opt(x)
				}
				return x
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}

			loadEnvs()

			assert.Equal(t, tc.expect, GetEncoder())

			for k := range tc.envs {
				os.Unsetenv(k)
			}

			t.Cleanup(func() {
				// reset them
				DefaultEncoding = LineProtocol
			})
		})
	}
}

// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"errors"
	"fmt"
	"runtime"
	T "testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPutGetConcurrentRotation(t *T.T) {
	const records = 256

	c, err := Open(
		WithPath(t.TempDir()),
		WithBatchSize(1),
		WithNoPos(true),
		WithNoSync(true),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, c.Close())
	})

	start := make(chan struct{})
	results := make(chan error, 2)

	go func() {
		<-start
		for i := range records {
			if err := c.Put([]byte{byte(i)}); err != nil {
				results <- fmt.Errorf("put record %d: %w", i, err)
				return
			}
		}
		results <- nil
	}()

	go func() {
		<-start
		deadline := time.Now().Add(2 * time.Second)
		for next := 0; next < records; {
			err := c.Get(func(got []byte) error {
				if len(got) != 1 || got[0] != byte(next) {
					return fmt.Errorf("record %d: got %v", next, got)
				}
				next++
				return nil
			})
			if err == nil {
				continue
			}
			if !errors.Is(err, ErrNoData) {
				results <- fmt.Errorf("get record %d: %w", next, err)
				return
			}
			if time.Now().After(deadline) {
				results <- fmt.Errorf("get record %d: timed out", next)
				return
			}
			runtime.Gosched()
		}
		results <- nil
	}()

	close(start)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
}

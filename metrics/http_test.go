// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package metrics

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	T "testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoMetrics(t *T.T) {

	listen := fmt.Sprintf("0.0.0.0:%d", time.Now().UnixNano()%65535)
	ms := &MetricServer{
		URL:              "/metrics",
		Listen:           listen,
		Enable:           true,
		DisableGoMetrics: false,
	}

	go func() {
		require.NoError(t, ms.Start())
	}()

	time.Sleep(time.Second)

	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", listen))
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Truef(t, bytes.Contains(body, []byte("go_gc")), "body not contains `go_gc`, body:\n%s", string(body))
}

func TestRouteForGin(t *T.T) {
	t.Run("gin", func(t *T.T) {
		vec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: filepath.Base(t.Name()),
			},
			[]string{"api", "status"},
		)

		MustRegister(vec)

		router := gin.New()
		router.GET("/metrics", HTTPGinHandler(promhttp.HandlerOpts{}))
		ts := httptest.NewServer(router)
		defer ts.Close()

		div := 10000.0

		for i := 0; i < 1000; i++ {
			switch i % 3 {
			case 0:
				vec.WithLabelValues("/v1/write/metric", "ok").Add(float64(i) / div)
			case 1:
				vec.WithLabelValues("/v1/write/logging", "ok").Add(float64(i) / div)
			default:
				vec.WithLabelValues("/v1/write/tracing", "ok").Add(float64(i) / div)
			}
		}

		req, err := http.Get(fmt.Sprintf("%s/metrics", ts.URL))
		assert.NoError(t, err)

		body, err := io.ReadAll(req.Body)
		assert.NoError(t, err)

		t.Logf("\n%s", string(body))
	})
}

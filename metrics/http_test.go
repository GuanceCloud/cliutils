// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package metrics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	T "testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

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

		body, err := ioutil.ReadAll(req.Body)
		assert.NoError(t, err)

		t.Logf("\n%s", string(body))
	})
}

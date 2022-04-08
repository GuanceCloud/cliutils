package http

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	tu "gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"
)

type apiStat struct {
	total     int
	costTotal time.Duration
}

func TestHTTPWrapperWithMetricReporter(t *testing.T) {
	r := gin.New()

	limitRate := 10
	lmt := NewAPIRateLimiter(float64(limitRate))

	testHandler := func(http.ResponseWriter, *http.Request, ...interface{}) (interface{}, error) {
		return nil, nil
	}

	plg := &WrapPlugins{
		Limiter:  lmt,
		Reporter: &ReporterImpl{},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		StartReporter()
	}()

	r.GET("/test", HTTPAPIWrapper(plg, testHandler))

	ts := httptest.NewServer(r)
	defer ts.Close()

	time.Sleep(time.Second)

	var resp *http.Response
	var err error
	var body []byte
	for i := 0; i < limitRate*1000; i++ { // this should exceed max limit and got a 429 status code
		resp, err = http.Get(fmt.Sprintf("%s/test", ts.URL))
		if err != nil {
			t.Error(err)
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}

		resp.Body.Close()
	}

	tu.Equals(t, resp.StatusCode, 429)
	t.Logf("%s", string(body))

	stats := GetStats()
	for k, v := range stats {
		tu.Assert(t, v.Total == limitRate*1000, "expect %d == %d", v.Total, limitRate)
		tu.Assert(t, v.Limited > 0 && v.Status4XX > 0, "expect %d == %d", v.Total, limitRate)
		t.Logf("%s: %+#v", k, v)
	}

	StopReporter()
	wg.Wait()
}

func TestHTTPWrapperWithRateLimit(t *testing.T) {
	r := gin.New()

	limitRate := 10
	lmt := NewAPIRateLimiter(float64(limitRate))

	testHandler := func(http.ResponseWriter, *http.Request, ...interface{}) (interface{}, error) {
		return nil, nil
	}

	r.GET("/test", HTTPAPIWrapper(&WrapPlugins{
		Limiter: lmt,
	}, testHandler))

	ts := httptest.NewServer(r)
	defer ts.Close()

	time.Sleep(time.Second)

	var resp *http.Response
	var err error
	var body []byte
	for i := 0; i < limitRate*2; i++ { // this should exceed max limit and got a 429 status code
		resp, err = http.Get(fmt.Sprintf("%s/test", ts.URL))
		if err != nil {
			t.Error(err)
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}

		resp.Body.Close()
	}

	tu.Equals(t, resp.StatusCode, 429)
	t.Logf("%s", string(body))
}

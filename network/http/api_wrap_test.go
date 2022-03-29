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

type reporterMock struct {
	ch    chan *APIMetric
	stats map[string]*apiStat
}

func (r *reporterMock) Report(m *APIMetric) {
	r.ch <- m
}

func TestHTTPWrapperWithMetricReporter(t *testing.T) {
	r := gin.New()

	limitRate := 10
	lmt := NewRateLimiter(float64(limitRate))

	testHandler := func(http.ResponseWriter, *http.Request, ...interface{}) (interface{}, error) {
		return nil, nil
	}

	reporter := &reporterMock{
		ch:    make(chan *APIMetric),
		stats: map[string]*apiStat{},
	}

	plg := &Plugins{
		Limiter:  lmt,
		Reporter: reporter,
	}

	chexit := make(chan interface{})
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case m := <-reporter.ch:
				if _, ok := reporter.stats[m.API]; !ok {
					reporter.stats[m.API] = &apiStat{total: 1, costTotal: m.Latency}
				} else {
					reporter.stats[m.API].total++
				}
			case <-chexit:
				return
			}
		}
	}()

	r.GET("/test", HTTPAPIWrapper(plg, testHandler))

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
	close(chexit)
	wg.Wait()

	for k, v := range reporter.stats {
		tu.Equals(t, v.total, 20)
		t.Logf("%s: %+#v", k, v)
	}
}

func TestHTTPWrapperWithRateLimit(t *testing.T) {
	r := gin.New()

	limitRate := 10
	lmt := NewRateLimiter(float64(limitRate))

	testHandler := func(http.ResponseWriter, *http.Request, ...interface{}) (interface{}, error) {
		return nil, nil
	}

	r.GET("/test", HTTPAPIWrapper(&Plugins{
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

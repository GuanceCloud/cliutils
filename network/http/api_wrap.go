package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gin-gonic/gin"
)

var (
	ErrTooManyRequest = NewErr(errors.New("reach max API rate limit"), http.StatusTooManyRequests)
	HttpOK            = NewErr(nil, http.StatusOK)
	EnableTracing     bool
)

// RateLimiter used to define API request rate limiter
type RateLimiter interface {
	// detect if rate limit reached
	IsLimited(http.ResponseWriter, *http.Request) bool

	// If rate limited, do anything what you want(cache the request, or do nothing)
	LimitReadchedCallback(http.ResponseWriter, *http.Request)

	// Update rate limite exclusively
	UpdateRate(float64)
}

type APIMetric struct {
	API        string
	Latency    time.Duration
	StatusCode int
	Limited    bool
}

// APIMetric used to collects API metrics during API handing
type APIMetricReporter interface {
	Report(*APIMetric) // report these metrics
}

type WrapPlugins struct {
	Limiter  RateLimiter
	Reporter APIMetricReporter
	//Tracer   Tracer
}

// RateLimiterImpl is default implemented of RateLimiter based on tollbooth
type RateLimiterImpl struct {
	*limiter.Limiter
}

func NewRateLimiter(rate float64) *RateLimiterImpl {
	return &RateLimiterImpl{
		Limiter: tollbooth.NewLimiter(rate, &limiter.ExpirableOptions{
			DefaultExpirationTTL: time.Second,
		}).SetBurst(1),
	}
}

func (rl *RateLimiterImpl) IsLimited(w http.ResponseWriter, r *http.Request) bool {
	if rl.Limiter == nil {
		return false
	}

	return tollbooth.LimitByRequest(rl.Limiter, w, r) != nil
}

// LimitReadchedCallback do nothing, just drop the request
func (rl *RateLimiterImpl) LimitReadchedCallback(w http.ResponseWriter, r *http.Request) {
	// do nothing
}

// UpdateRate update limite rate exclusively
func (rl *RateLimiterImpl) UpdateRate(rate float64) {
	rl.Limiter.SetMax(rate)
}

type apiHandler func(http.ResponseWriter, *http.Request, ...interface{}) (interface{}, error)

func HTTPAPIWrapper(plugins *WrapPlugins, next apiHandler, any ...interface{}) func(*gin.Context) {
	return func(c *gin.Context) {
		var start time.Time
		var m *APIMetric

		if plugins.Reporter != nil {
			start = time.Now()
			m = &APIMetric{
				API: c.Request.URL.Path + "@" + c.Request.Method,
			}
		}

		if plugins.Limiter != nil {
			if plugins.Limiter.IsLimited(c.Writer, c.Request) {
				HttpErr(c, ErrTooManyRequest)
				plugins.Limiter.LimitReadchedCallback(c.Writer, c.Request)
				if m != nil {
					m.StatusCode = ErrTooManyRequest.HttpCode
					m.Limited = true
				}
				c.Abort()
				goto feed
			}
		}

		if res, err := next(c.Writer, c.Request, any...); err != nil {
			HttpErr(c, err)
		} else {
			HttpOK.WriteBody(c, res)
		}

		if m != nil {
			m.StatusCode = c.Writer.Status()
			m.Latency = time.Since(start)
		}

	feed:
		if m != nil {
			plugins.Reporter.Report(m)
		}
	}
}

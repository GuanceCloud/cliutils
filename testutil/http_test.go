// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package testutil

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHTTPServer(t *testing.T) {
	opt := &HTTPServerOptions{
		Bind: ":12345",
		Exit: make(chan interface{}),
		Routes: map[string]func(*gin.Context){
			"/route1": func(*gin.Context) { fmt.Printf("on route1") },
			"/route2": func(*gin.Context) { fmt.Printf("on route2") },
		},
	}

	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()
		NewHTTPServer(t, opt)
	}()

	time.Sleep(time.Second)

	_, err := http.Get("http://:12345/route1")
	if err != nil {
		t.Error(err)
	}

	_, err = http.Post("http://:12345/route2", "", nil)
	if err != nil {
		t.Error(err)
	}

	close(opt.Exit)
	wg.Wait()
}

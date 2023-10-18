// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package refertable

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckUrl(t *testing.T) {
	cases := []struct {
		url    string
		scheme string
		fail   bool
	}{
		{
			url:    "ht\tp//localss(",
			scheme: "http",
			fail:   true,
		},
		{
			url:    "httpS://localss(",
			scheme: "https",
			fail:   true,
		},
		{
			url:    "https://localhost/aa?a",
			scheme: "https",
		},
		{
			url:    "http://localhost/aa?a",
			scheme: "http",
		},
		{
			url:    "oss://localhost/aa?a",
			scheme: "oss",
			fail:   true,
		},
	}

	for _, v := range cases {
		scheme, err := checkURL(v.url)
		if err != nil {
			if !v.fail {
				t.Error(err)
			}
			continue
		}
		assert.Equal(t, v.scheme, scheme)
	}
}

func TestRunner(t *testing.T) {
	files := map[string]string{
		"a.json": testTableData,
	}
	server := newJSONDataServer(files)
	defer server.Close()

	url := server.URL

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	refT, err := NewReferTable(RefTbCfg{
		URL:      url + "?name=a.json",
		Interval: time.Second * 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	ok := refT.InitFinished(time.Second)
	if ok {
		t.Fatal("should not be done")
	}

	go refT.PullWorker(ctx)

	ok = refT.InitFinished(time.Second)
	if !ok {
		t.Fatal("init not finishd")
	}

	tables := refT.Tables()
	if tables == nil {
		t.Fatal("refT == nil")
	}

	v, ok := tables.Query("table1", []string{"key1"}, []any{"a"}, nil)
	if !ok || len(v) == 0 {
		t.Error("!ok")
	}

	stats := tables.Stats()
	assert.Equal(t, stats.Name[0], "table1")
	assert.Equal(t, stats.Name[1], "table2")
	assert.Equal(t, stats.Row[0], 3)
	assert.Equal(t, stats.Row[1], 2)
}

func newJSONDataServer(files map[string]string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
			default:
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			name := r.FormValue("name")
			data := files[name]
			w.Write([]byte(data))
			w.WriteHeader(http.StatusOK)
		},
	))
	return server
}

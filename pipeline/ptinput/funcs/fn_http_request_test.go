package funcs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/point"
	tu "github.com/GuanceCloud/cliutils/testutil"
)

func TestHTTPRequest(t *testing.T) {
	server := HTTPServer()
	defer server.Close()

	url := `"` + server.URL + "/testResp" + `"`
	fmt.Println(url)

	cases := []struct {
		name, pl, in string
		expected     interface{}
		fail         bool
		outkey       string
	}{
		{
			name: "acquire_code",
			pl: `resp = http_request("GET", ` + url + `) 
			add_key(abc, resp["status_code"])`,
			in:       `[]`,
			expected: int64(200),
			outkey:   "abc",
		},
		{
			name: "acquire_body_without_headers",
			pl: `resp = http_request("GET", ` + url + `)
			resp_body = load_json(resp["body"])
			add_key(abc, resp_body["a"])`,
			in:       `[]`,
			expected: "hello",
			outkey:   `abc`,
		},
		{
			name: "acquire_body_with_headers",
			pl: `resp = http_request("GET", ` + url + `, {"extraHeader1": "1", "extraHeader2": "1"})
			resp_body = load_json(resp["body"])
			add_key(abc, resp_body["a"])`,
			in:       `[]`,
			expected: "hello world",
			outkey:   `abc`,
		},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := NewTestingRunner(tc.pl)
			if err != nil {
				if tc.fail {
					t.Logf("[%d]expect error: %s", idx, err)
				} else {
					t.Errorf("[%d] failed: %s", idx, err)
				}
				return
			}
			pt := ptinput.NewPlPoint(
				point.Logging, "test", nil, map[string]any{"message": tc.in}, time.Now())
			errR := runScript(runner, pt)

			if errR != nil {
				t.Fatal(errR.Error())
			}

			v, _, _ := pt.Get(tc.outkey)
			// tu.Equals(t, nil, err)
			tu.Equals(t, tc.expected, v)

			t.Logf("[%d] PASS", idx)
		})
	}
}

func HTTPServer() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var responseData map[string]string
			headers := r.Header
			if headers.Get("extraHeader1") != "" && headers.Get("extraHeader2") != "" {
				responseData = map[string]string{"a": "hello world"}
			} else {
				responseData = map[string]string{"a": "hello"}
			}
			responseJSON, err := json.Marshal(responseData)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			w.Write(responseJSON)
			w.WriteHeader(http.StatusOK)
		},
	))
	return server
}

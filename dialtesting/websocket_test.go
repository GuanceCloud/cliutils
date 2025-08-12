// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"text/template"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var cases = []struct {
	t         *WebsocketTask
	fail      bool
	reasonCnt int
}{
	{
		fail:      false,
		reasonCnt: 0,
		t: &WebsocketTask{
			SuccessWhen: []*WebsocketSuccess{
				{
					ResponseTime: []*WebsocketResponseTime{{
						Target: "10s",
					}},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "success",
			},
		},
	},
	{
		fail:      false,
		reasonCnt: 1,
		t: &WebsocketTask{
			SuccessWhen: []*WebsocketSuccess{
				{
					ResponseTime: []*WebsocketResponseTime{{
						Target: "1us",
					}},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "response_time_large",
			},
		},
	},
}

func TestWebsocket(t *testing.T) {
	for _, c := range cases {
		server := websocketServer()
		defer server.Close()

		urlParsed, _ := url.Parse(server.URL)

		urlParsed.Scheme = "ws"
		c.t.URL = urlParsed.String()

		c.t.SetChild(c.t)

		if err := c.t.Check(); err != nil {
			if c.fail == false {
				t.Errorf("case: %s, failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		err := c.t.Run()
		if err != nil {
			if c.fail == false {
				t.Errorf("case %s failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		tags, fields := c.t.GetResults()

		t.Logf("ts: %+#v \n fs: %+#v \n ", tags, fields)

		reasons, _ := c.t.CheckResult()
		if len(reasons) != c.reasonCnt {
			t.Errorf("case %s expect %d reasons, but got %d reasons:\n\t%s",
				c.t.Name, c.reasonCnt, len(reasons), strings.Join(reasons, "\n\t"))
		} else if len(reasons) > 0 {
			t.Logf("case %s reasons:\n\t%s",
				c.t.Name, strings.Join(reasons, "\n\t"))
		}
	}
}

func websocketServer() *httptest.Server {
	upgrader := websocket.Upgrader{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// username, password, ok := r.BasicAuth()
		// fmt.Println(username, password, ok)
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				break
			}
			err = c.WriteMessage(mt, message)
			if err != nil {
				fmt.Println(err)
				break
			}
		}
	}))

	return ts
}

func TestWebsocketRenderTemplate(t *testing.T) {
	ct := &WebsocketTask{
		URL:     "{{url}}",
		Message: "{{message}}",
		SuccessWhen: []*WebsocketSuccess{
			{
				ResponseTime: []*WebsocketResponseTime{{
					Target: "10s",
				}},
			},
			{
				ResponseMessage: []*SuccessOption{{
					Is: "{{response_message}}",
				}},
			},
			{
				Header: map[string][]*SuccessOption{
					"header": {{
						Is: "{{header}}",
					}},
				},
			},
		},
		AdvanceOptions: &WebsocketAdvanceOption{
			RequestOptions: &WebsocketOptRequest{
				Headers: map[string]string{
					"header": "{{header}}",
				},
			},
			Auth: &WebsocketOptAuth{
				Username: "{{username}}",
				Password: "{{password}}",
			},
		},
	}

	fm := template.FuncMap{
		"url": func() string {
			return "url"
		},
		"header": func() string {
			return "header"
		},
		"username": func() string {
			return "username"
		},
		"password": func() string {
			return "password"
		},
		"port": func() string {
			return "8080"
		},
		"message": func() string {
			return "message"
		},
		"response_message": func() string {
			return "response_message"
		},
	}

	task, err := NewTask("", ct)
	assert.NoError(t, err)

	ct, ok := task.(*WebsocketTask)
	assert.True(t, ok)

	assert.NoError(t, ct.renderTemplate(fm))
	assert.Equal(t, "header", ct.AdvanceOptions.RequestOptions.Headers["header"])
	assert.Equal(t, "message", ct.Message)
	assert.Equal(t, "response_message", ct.SuccessWhen[1].ResponseMessage[0].Is)
	assert.Equal(t, "username", ct.AdvanceOptions.Auth.Username)
	assert.Equal(t, "password", ct.AdvanceOptions.Auth.Password)
	assert.Equal(t, "url", ct.URL)
}

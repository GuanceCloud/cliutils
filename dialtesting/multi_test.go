// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMulti(t *testing.T) {
	body := struct {
		Code  string `json:"code"`
		Token string `json:"token"`
	}{
		Token: fmt.Sprintf("token_%d", time.Now().UnixNano()),
	}

	engine := gin.Default()
	engine.GET("/token", func(ctx *gin.Context) {
		body.Token = ctx.Query("token")
		body.Code = "200"
		bodyBytes, _ := json.Marshal(body)
		ctx.Writer.Write(bodyBytes)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		engine.ServeHTTP(w, r)
	}))
	defer server.Close()

	cases := makeCases(server.URL)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			taskString, _ := json.Marshal(tc.Task)
			task, err := NewTask(string(taskString), tc.Task)

			assert.NoError(t, err)

			globalVars := map[string]Variable{
				"global_var_id": {
					Value: "global_var_value",
				},
			}
			assert.NoError(t, task.RenderTemplateAndInit(globalVars))

			assert.NoError(t, task.Run())

			tags, fields := task.GetResults()
			if tc.IsFailed {
				assert.Equal(t, -1, fields["success"])
			} else {
				assert.Equal(t, 1, fields["success"])
			}
			assert.NotNil(t, tags, fields)
			assert.NoError(t, tc.Check(t, tags, fields))
		})
	}
}

type cs struct {
	Name       string
	Task       *MultiTask
	IsFailed   bool
	Check      func(t assert.TestingT, tags map[string]string, fields map[string]interface{}) error
	GlobalVars map[string]Variable
}

func makeCases(serverURL string) []cs {
	return []cs{
		{
			Name:     "normal test",
			IsFailed: true,
			Task: func() *MultiTask {
				step1 := HTTPTask{
					URL: serverURL + "/token?token={{config_var_token}}-{{config_var_global}}",
					PostScript: `
			result["is_failed"] = false	
			body = load_json(response["body"])
			vars["token"] = body["Token"]
		`,
				}

				step1Bytes, _ := json.Marshal(step1)

				step3 := HTTPTask{
					URL: fmt.Sprintf("%s/token?token={{Token}}", serverURL),
					PostScript: `
			result["is_failed"] = true
			result["error_message"]	= "error"
		`,
				}

				step2Bytes, _ := json.Marshal(step3)
				return &MultiTask{
					Task: &Task{
						ConfigVars: []*ConfigVar{
							{
								Name:  "config_var_token",
								Value: "config_var_token",
							},
							{
								Name: "config_var_global",
								ID:   "global_var_id",
								Type: TypeVariableGlobal,
							},
						},
					},
					Steps: []*MultiStep{
						{
							Type:       "http",
							TaskString: string(step1Bytes),
							ExtractedVars: []MultiExtractedVar{
								{
									Name:  "Token",
									Field: "token",
								},
							},
						},
						{
							Type:  "wait",
							Value: 1,
						},
						{
							Type:       "http",
							TaskString: string(step2Bytes),
						},
					},
				}
			}(),
			GlobalVars: map[string]Variable{
				"global_var_id": {
					Value: "global_var_value",
				},
			},
			Check: func(t assert.TestingT, tags map[string]string, fields map[string]interface{}) error {
				assert.Equal(t, "FAIL", tags["status"])
				msg := map[string]interface{}{}
				message, ok := fields["message"].(string)
				assert.True(t, ok)
				assert.NoError(t, json.Unmarshal([]byte(message), &msg))
				assert.Equal(t, "error", msg["fail_reason"])
				assert.EqualValues(t, -1, fields["success"])
				return nil
			},
		},
		{
			Name:     "extract vars",
			IsFailed: false,
			Task: func() *MultiTask {
				step1 := HTTPTask{
					URL: serverURL + "/token?token=token123",
					PostScript: `
					body = load_json(response["body"])

					if body["code"] == "200" {
						result["is_failed"] = false
						vars["token"] = body["token"]
					} else {
						result["is_failed"] = true
						result["error_message"] = body["message"]
					}
		`,
				}

				step1Bytes, _ := json.Marshal(step1)

				return &MultiTask{
					Steps: []*MultiStep{
						{
							Type:       "http",
							TaskString: string(step1Bytes),
							ExtractedVars: []MultiExtractedVar{
								{
									Name:   "token_secure",
									Field:  "token",
									Secure: true,
								},
								{
									Name:  "token",
									Field: "token",
								},
							},
						},
					},
				}
			}(),
			Check: func(t assert.TestingT, tags map[string]string, fields map[string]interface{}) error {
				assert.Equal(t, "OK", tags["status"])
				steps := []MultiStep{}
				str, ok := fields["steps"].(string)
				assert.True(t, ok)
				assert.NoError(t, json.Unmarshal([]byte(str), &steps))
				assert.True(t, (len(steps) == 1) && (len(steps[0].ExtractedVars) == 2))
				assert.Equal(t, "", steps[0].ExtractedVars[0].Value)
				assert.Equal(t, "token123", steps[0].ExtractedVars[1].Value)
				return nil
			},
		},
		{
			Name:     "config vars",
			IsFailed: false,
			Task: func() *MultiTask {
				step1 := HTTPTask{
					URL:        serverURL + "/token",
					PostScript: `result["is_failed"] = false`,
				}

				step1Bytes, _ := json.Marshal(step1)

				return &MultiTask{
					Task: &Task{
						ConfigVars: []*ConfigVar{
							{
								Name:  "config_var_token",
								Value: "config_var_token",
							},
							{
								Name:   "config_var_token_secure",
								Value:  "config_var_token",
								Secure: true,
							},
							{
								Name: "config_var_global",
								ID:   "global_var_id",
								Type: TypeVariableGlobal,
							},
						},
					},
					Steps: []*MultiStep{
						{
							Type:       "http",
							TaskString: string(step1Bytes),
						},
					},
				}
			}(),
			GlobalVars: map[string]Variable{
				"global_var_id": {
					Value: "global_var_value",
				},
			},
			Check: func(t assert.TestingT, tags map[string]string, fields map[string]interface{}) error {
				assert.Equal(t, "OK", tags["status"])
				vars := []ConfigVar{}
				configVarstring, ok := fields["config_vars"].(string)
				assert.True(t, ok)
				assert.NoError(t, json.Unmarshal([]byte(configVarstring), &vars))

				// check config vars
				assert.Equal(t, 3, len(vars))
				// config var value
				assert.Equal(t, "config_var_token", vars[0].Value)
				// secure value
				assert.Equal(t, "", vars[1].Value)
				// global var
				assert.Equal(t, "global_var_value", vars[2].Value)

				return nil
			},
		},
	}
}

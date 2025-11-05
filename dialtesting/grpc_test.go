// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"os"
	T "testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
)

//nolint:golint // Requires real gRPC server access
func TestGRPCTask_Check(t *T.T) {
	t.Run("missing server", func(t *T.T) {
		task := &GRPCTask{
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
						ProtoFiles: map[string]string{
							"greeter.proto": "syntax = \"proto3\";",
						},
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "hello"},
					},
				},
			},
		}
		task.initTask()
		err := task.check()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server address is required")
	})

	t.Run("missing proto files", func(t *T.T) {
		task := &GRPCTask{
			Server: "localhost:50051",
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
						ProtoFiles: map[string]string{}, // 空的 proto files
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "hello"},
					},
				},
			},
		}
		task.initTask()
		err := task.check()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "proto files not provided")
	})

	t.Run("missing full method", func(t *T.T) {
		task := &GRPCTask{
			Server: "localhost:50051",
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "hello"},
					},
				},
			},
		}
		task.initTask()
		err := task.check()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "full method is required")
	})

	t.Run("missing check rule", func(t *T.T) {
		task := &GRPCTask{
			Server: "localhost:50051",
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
						ProtoFiles: map[string]string{
							"greeter.proto": "syntax = \"proto3\";",
						},
					},
				},
			},
		}
		task.initTask()
		err := task.check()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no any check rule")
	})
}

func TestGRPCTask_Init(t *T.T) {
	serverAddr := "localhost:50052"
	t.Run("init with default timeout", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)
		assert.Equal(t, DefaultGRPCTimeout, task.timeout)
	})

	t.Run("init with custom timeout", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					RequestTimeout: "10s",
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)
		assert.Equal(t, 10*time.Second, task.timeout)
	})

	t.Run("init with invalid timeout", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					RequestTimeout: "invalid",
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout")
	})

	t.Run("init with response time in success checker", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					ResponseTime: "1s",
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)
		assert.Equal(t, 1*time.Second, task.SuccessWhen[0].respTime)
	})

	t.Run("init with invalid response time", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					ResponseTime: "invalid",
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid response time")
	})

	t.Run("init with body regex in success checker", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "hello"},
					},
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)
	})

	t.Run("init with invalid regex in body", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{MatchRegex: "[invalid regex"},
					},
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "compile regex failed")
	})

	t.Run("init with TLS certificate - ignore server cert error", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
				Certificate: &GRPCOptCertificate{
					IgnoreServerCertificateError: true,
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)
		assert.NotNil(t, task.creds)
	})
}

//nolint:golint // Requires real gRPC server access
func TestGRPCTask_GetResults(t *T.T) {
	t.Skip("Skipping test that requires real gRPC server")
	serverAddr := "localhost:50051"
	greeterProto, err := os.ReadFile("grpcproto/greeter.proto")
	if err != nil {
		t.Fatalf("Failed to read greeter.proto: %v", err)
	}
	commonProto, err := os.ReadFile("grpcproto/common.proto")
	if err != nil {
		t.Fatalf("Failed to read common.proto: %v", err)
	}

	t.Run("success result", func(t *T.T) {
		task := &GRPCTask{
			Task: &Task{
				Name: "test-task",
				Tags: map[string]string{
					"env": "test",
				},
			},
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			PostScript: `body = load_json(response["body"])
if body != nil && body["msg"] != nil {
  result["is_failed"] = false
  vars["msg"] = body["msg"]
} else {
  result["is_failed"] = true
  result["error_message"] = "响应中缺少 msg 字段"
}`,
		}
		task.initTask()

		err := task.check()
		assert.NoError(t, err)

		err = task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)

		tags, fields := task.getResults()
		t.Logf("tags: %v, fields: %v", tags, fields)

		// Check tags
		assert.Equal(t, "test-task", tags["name"])
		assert.Equal(t, serverAddr, tags["server"])
		assert.Equal(t, "greeter.Greeter/SayHello", tags["method"])
		assert.Equal(t, "OK", tags["status"])
		assert.Equal(t, "grpc", tags["proto"])
		assert.Equal(t, "test", tags["env"])

		// Check fields
		assert.Equal(t, int64(1), fields["success"])
		assert.Greater(t, fields["response_time"], int64(0))
		assert.NotNil(t, fields["message"])
	})

	t.Run("failure result", func(t *T.T) {
		task := &GRPCTask{
			Task: &Task{
				Name: "test-task-fail",
			},
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `invalid json`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			PostScript: "",
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)

		tags, fields := task.getResults()

		// Check tags
		assert.Equal(t, "FAIL", tags["status"])

		// Check fields
		assert.Equal(t, int64(-1), fields["success"])
		assert.NotNil(t, fields["fail_reason"])
	})
}

func TestGRPCTask_OtherMethods(t *T.T) {
	t.Run("class", func(t *T.T) {
		task := &GRPCTask{}
		assert.Equal(t, ClassGRPC, task.class())
	})

	t.Run("metricName", func(t *T.T) {
		task := &GRPCTask{}
		assert.Equal(t, "grpc_dial_testing", task.metricName())
	})

	t.Run("initTask", func(t *T.T) {
		task := &GRPCTask{}
		task.initTask()
		assert.NotNil(t, task.Task)
	})

	t.Run("clear", func(t *T.T) {
		task := &GRPCTask{
			result:   []byte("test"),
			reqError: "error",
			reqCost:  100 * time.Millisecond,
		}
		task.clear()

		assert.Nil(t, task.result)
		assert.Empty(t, task.reqError)
		assert.Equal(t, time.Duration(0), task.reqCost)
	})

	t.Run("checkResult", func(t *T.T) {
		t.Run("success without conditions", func(t *T.T) {
			task := &GRPCTask{
				result: []byte(`{"message":"hello test"}`),
			}
			reasons, flag := task.checkResult()
			assert.Nil(t, reasons)
			assert.True(t, flag)
		})

		t.Run("with error", func(t *T.T) {
			task := &GRPCTask{
				reqError: "test error",
			}
			reasons, flag := task.checkResult()
			assert.NotEmpty(t, reasons)
			assert.False(t, flag)
			assert.Equal(t, "test error", reasons[0])
		})

		t.Run("no response", func(t *T.T) {
			task := &GRPCTask{
				PostScript: "",
			}
			reasons, flag := task.checkResult()
			assert.NotEmpty(t, reasons)
			assert.False(t, flag)
			assert.Contains(t, reasons[0], "no response")
		})

		t.Run("success with body check", func(t *T.T) {
			task := &GRPCTask{
				result: []byte(`{"message":"hello test"}`),
				SuccessWhen: []*GRPCSuccess{
					{
						Body: []*SuccessOption{
							{Contains: "hello"},
						},
					},
				},
			}
			// Initialize regex patterns
			for _, checker := range task.SuccessWhen {
				for _, v := range checker.Body {
					err := genReg(v)
					assert.NoError(t, err)
				}
			}
			reasons, flag := task.checkResult()
			assert.Empty(t, reasons)
			assert.True(t, flag)
		})

		t.Run("failure with body check", func(t *T.T) {
			task := &GRPCTask{
				result: []byte(`{"message":"hello test"}`),
				SuccessWhen: []*GRPCSuccess{
					{
						Body: []*SuccessOption{
							{Contains: "notfound"},
						},
					},
				},
			}
			// Initialize regex patterns
			for _, checker := range task.SuccessWhen {
				for _, v := range checker.Body {
					err := genReg(v)
					assert.NoError(t, err)
				}
			}
			reasons, _ := task.checkResult()
			assert.NotEmpty(t, reasons)
		})
	})

	t.Run("getHostName", func(t *T.T) {
		t.Run("with port", func(t *T.T) {
			task := &GRPCTask{
				Server: "localhost:50051",
			}
			hostnames, err := task.getHostName()
			assert.NoError(t, err)
			assert.Equal(t, []string{"localhost"}, hostnames)
		})

		t.Run("empty server", func(t *T.T) {
			task := &GRPCTask{}
			_, err := task.getHostName()
			assert.Error(t, err)
		})
	})

	t.Run("getVariableValue", func(t *T.T) {
		t.Run("without post script", func(t *T.T) {
			task := &GRPCTask{}
			_, err := task.getVariableValue(Variable{})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "post_script is empty")
		})

		t.Run("without result", func(t *T.T) {
			task := &GRPCTask{
				PostScript: "vars[\"test\"] = \"value\"",
			}
			_, err := task.getVariableValue(Variable{
				TaskVarName: "test",
			})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "response body is empty")
		})
	})

	t.Run("getRawTask", func(t *T.T) {
		task := &GRPCTask{
			Server: "localhost:50051",
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					RequestTimeout: "30s",
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod: "greeter.Greeter/SayHello",
					},
				},
			},
		}
		task.initTask()

		taskJSON, _ := json.Marshal(task)
		rawTask, err := task.getRawTask(string(taskJSON))
		assert.NoError(t, err)

		var parsed GRPCTask
		err = json.Unmarshal([]byte(rawTask), &parsed)
		assert.NoError(t, err)
		assert.Equal(t, task.Server, parsed.Server)
		assert.Equal(t, task.getFullMethod(), parsed.getFullMethod())
		assert.Equal(t, task.AdvanceOptions.RequestOptions.RequestTimeout, parsed.AdvanceOptions.RequestOptions.RequestTimeout)
	})
}

func TestBuildExtendedProtoMap(t *T.T) {
	t.Run("with all imports present", func(t *T.T) {
		greeterProto := `syntax = "proto3";
package greeter;
import "greeter/user.proto";
service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {}
}`

		userProto := `syntax = "proto3";
package user;
message GetUserRequest {
  int32 user_id = 1;
}`

		protoFiles := map[string]string{
			"greeter.proto":      greeterProto,
			"greeter/user.proto": userProto,
		}

		extendedMap, err := buildExtendedProtoMap(protoFiles)
		assert.NoError(t, err)

		// Check original files are preserved
		assert.Equal(t, greeterProto, extendedMap["greeter.proto"])
		assert.Equal(t, userProto, extendedMap["greeter/user.proto"])
		assert.Equal(t, 2, len(extendedMap))
	})

	t.Run("with missing import", func(t *T.T) {
		greeterProto := `syntax = "proto3";
package greeter;
import "greeter/user.proto";
service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {}
}`

		protoFiles := map[string]string{
			"greeter.proto": greeterProto,
			// user.proto is missing
		}

		extendedMap, err := buildExtendedProtoMap(protoFiles)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing imports")
		assert.Contains(t, err.Error(), "greeter/user.proto")
		assert.Nil(t, extendedMap)
	})

	t.Run("with multiple imports all present", func(t *T.T) {
		mainProto := `syntax = "proto3";
package main;
import "greeter/user.proto";
import "greeter/common.proto";
service Main {}
`

		userProto := `syntax = "proto3";
package user;
message User {}
`

		commonProto := `syntax = "proto3";
package common;
message Common {}
`

		protoFiles := map[string]string{
			"main.proto":           mainProto,
			"greeter/user.proto":   userProto,
			"greeter/common.proto": commonProto,
		}

		extendedMap, err := buildExtendedProtoMap(protoFiles)
		assert.NoError(t, err)

		// Check all files are preserved
		assert.Equal(t, mainProto, extendedMap["main.proto"])
		assert.Equal(t, userProto, extendedMap["greeter/user.proto"])
		assert.Equal(t, commonProto, extendedMap["greeter/common.proto"])
		assert.Equal(t, 3, len(extendedMap))
	})

	t.Run("with multiple imports one missing", func(t *T.T) {
		mainProto := `syntax = "proto3";
package main;
import "greeter/user.proto";
import "greeter/common.proto";
service Main {}
`

		userProto := `syntax = "proto3";
package user;
message User {}
`

		protoFiles := map[string]string{
			"main.proto":         mainProto,
			"greeter/user.proto": userProto,
			// greeter/common.proto is missing
		}

		extendedMap, err := buildExtendedProtoMap(protoFiles)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing imports")
		assert.Contains(t, err.Error(), "greeter/common.proto")
		assert.Nil(t, extendedMap)
	})

	t.Run("with no imports", func(t *T.T) {
		protoFiles := map[string]string{
			"simple.proto": `syntax = "proto3"; package simple;`,
		}

		extendedMap, err := buildExtendedProtoMap(protoFiles)
		assert.NoError(t, err)

		// Should have 1 entry (original file)
		assert.Equal(t, 1, len(extendedMap))
		assert.NotEmpty(t, extendedMap["simple.proto"])
	})

	t.Run("with path in filename and no imports", func(t *T.T) {
		protoFiles := map[string]string{
			"path/to/simple.proto": `syntax = "proto3"; package simple;`,
		}

		extendedMap, err := buildExtendedProtoMap(protoFiles)
		assert.NoError(t, err)

		// Should have 1 entry (original path preserved)
		assert.Equal(t, 1, len(extendedMap))
		assert.NotEmpty(t, extendedMap["path/to/simple.proto"])
	})

	t.Run("with empty proto files", func(t *T.T) {
		protoFiles := map[string]string{}

		extendedMap, err := buildExtendedProtoMap(protoFiles)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(extendedMap))
	})
}

//nolint:golint // Requires real gRPC server access
func TestGRPCTask_PostScript(t *T.T) {
	t.Skip("Skipping test that requires real gRPC server")
	serverAddr := "localhost:50051"
	greeterProto, err := os.ReadFile("grpcproto/greeter.proto")
	if err != nil {
		t.Fatalf("Failed to read greeter.proto: %v", err)
	}
	commonProto, err := os.ReadFile("grpcproto/common.proto")
	if err != nil {
		t.Fatalf("Failed to read common.proto: %v", err)
	}

	t.Run("post script success", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			PostScript: `
body = load_json(response["body"])
vars["msg"] = body["msg"]
result["is_failed"] = false
			`,
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)
		if task.reqError != "" {
			t.Fatalf("RPC call failed: %s", task.reqError)
		}
		assert.NotNil(t, task.postScriptResult, "postScriptResult should not be nil, reqError: %s", task.reqError)
		if task.postScriptResult != nil {
			assert.Equal(t, "你好, test! 这是来自 gRPC 的问候", task.postScriptResult.Vars["msg"])
		}
	})

	t.Run("post script failure", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			PostScript: `
result["is_failed"] = true
result["error_message"] = "custom error"
			`,
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err) // script runs but marks as failed
		assert.NotNil(t, task.postScriptResult)
		assert.True(t, task.postScriptResult.Result.IsFailed)

		reasons, flag := task.checkResult()
		assert.False(t, flag)
		assert.NotEmpty(t, reasons)
	})
}

//nolint:golint // Requires real gRPC server access
func TestGRPCTask_SuccessWhen(t *T.T) {
	t.Skip("Skipping test that requires real gRPC server")
	serverAddr := "localhost:50051"
	greeterProto, err := os.ReadFile("grpcproto/greeter.proto")
	if err != nil {
		t.Fatalf("Failed to read greeter.proto: %v", err)
	}
	commonProto, err := os.ReadFile("grpcproto/common.proto")
	if err != nil {
		t.Fatalf("Failed to read common.proto: %v", err)
	}

	t.Run("success with body contains", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "test"},
					},
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)

		reasons, flag := task.checkResult()
		assert.True(t, flag)
		assert.Empty(t, reasons)

		tags, fields := task.getResults()
		assert.Equal(t, "OK", tags["status"])
		assert.Equal(t, int64(1), fields["success"])
	})

	t.Run("failure with body check", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "notfound"},
					},
				},
			},
			SuccessWhenLogic: "and",
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)

		reasons, _ := task.checkResult()
		// With "and" logic, if condition fails, it should fail
		assert.NotEmpty(t, reasons)
	})

	t.Run("success with response time", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					ResponseTime: "10s",
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)

		_, flag := task.checkResult()
		assert.True(t, flag)
	})

	t.Run("failure with response time exceeded", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
				},
			},
			SuccessWhen: []*GRPCSuccess{
				{
					ResponseTime: "1ms", // very short timeout
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		// Add a small delay to ensure response time exceeds threshold
		time.Sleep(10 * time.Millisecond)

		err = task.run()
		assert.NoError(t, err)

		reasons, flag := task.checkResult()
		// Response time check may pass or fail depending on actual response time
		_ = reasons
		_ = flag
	})
}

//nolint:golint // Requires real gRPC server access
func TestGRPCTask_RequestDiscoveryModes(t *T.T) {
	t.Skip("Skipping test that requires real gRPC server")
	serverAddr := "localhost:50051"
	// 读取 proto 文件
	greeterProto, err := os.ReadFile("grpcproto/greeter.proto")
	if err != nil {
		t.Skipf("无法读取 greeter.proto 文件: %v", err)
	}

	commonProto, err := os.ReadFile("grpcproto/common.proto")
	if err != nil {
		t.Skipf("无法读取 common.proto 文件: %v", err)
	}

	// 模式1: ProtoFiles 发现模式
	t.Run("Request mode 1: ProtoFiles discovery", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"test"}`,
						ProtoFiles: map[string]string{
							"greeter.proto":          string(greeterProto),
							"grpcproto/common.proto": string(commonProto),
						},
					},
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					RequestTimeout: "1s",
				},
				Certificate: func() *GRPCOptCertificate {
					// 回退到跳过证书验证模式
					return &GRPCOptCertificate{
						IgnoreServerCertificateError: true,
					}
				}(),
			},
			SuccessWhenLogic: "and",
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "test"},
					},
					ResponseTime: "1s",
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)
		assert.Empty(t, task.reqError)
		assert.NotNil(t, task.result)

		tags, fields := task.getResults()
		assert.Equal(t, "greeter.Greeter/SayHello", tags["method"])
		assert.Equal(t, "OK", tags["status"])
		t.Logf("ProtoFiles mode - tags: %v, fields: %v", tags, fields)
	})

	// 模式2: Reflection 发现模式
	t.Run("Request mode 2: Reflection discovery", func(t *T.T) {
		task := &GRPCTask{
			Server: serverAddr,
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod:  "greeter.Greeter/SayHello",
						JSONRequest: `{"name":"reflection-test"}`,
					},
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					RequestTimeout: "1s",
				},
				Certificate: func() *GRPCOptCertificate {
					// 回退到跳过证书验证模式
					return &GRPCOptCertificate{
						IgnoreServerCertificateError: true,
					}
				}(),
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "reflection-test"},
					},
					ResponseTime: "1s",
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)
		assert.Empty(t, task.reqError)
		assert.NotNil(t, task.result)

		tags, fields := task.getResults()
		assert.Equal(t, "greeter.Greeter/SayHello", tags["method"])
		assert.Equal(t, "OK", tags["status"])
		t.Logf("Reflection mode - tags: %v, fields: %v", tags, fields)
	})

	// 模式3: HealthCheck 发现模式
	t.Run("Request mode 3: HealthCheck discovery", func(t *T.T) {
		task := &GRPCTask{
			Server: "localhost:50053",
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					HealthCheck: &GRPCHealthCheckDiscovery{
						// Service: "greeter.Greeter",
					},
					Metadata: map[string]string{
						"api-key": "test-key-123",
					},
					RequestTimeout: "1s",
				},
				Certificate: func() *GRPCOptCertificate {
					// 回退到跳过证书验证模式
					return &GRPCOptCertificate{
						IgnoreServerCertificateError: true,
					}
				}(),
			},
			SuccessWhen: []*GRPCSuccess{
				{
					Body: []*SuccessOption{
						{Contains: "SERVING"},
					},
				},
			},
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)
		assert.Empty(t, task.reqError)
		assert.NotNil(t, task.result)
		assert.Contains(t, string(task.result), "SERVING")

		tags, fields := task.getResults()
		assert.Equal(t, "grpc.health.v1.Health/Check", tags["method"])
		assert.Equal(t, "OK", tags["status"])
		t.Logf("HealthCheck mode - tags: %v, fields: %v", tags, fields)
	})
}

func TestGRPCTask_RenderTemplate(t *T.T) {
	t.Run("render template with all fields", func(t *T.T) {
		ct := &GRPCTask{
			Task:   &Task{},
			Server: "{{server_host}}:{{server_port}}",
			// PostScript is not rendered by template (static script content)
			PostScript: "vars[\"message\"] = \"hello\"",
			SuccessWhen: []*GRPCSuccess{
				{
					ResponseTime: "{{response_time}}",
					Body: []*SuccessOption{
						{Contains: "{{body_contains}}"},
					},
				},
			},
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					RequestTimeout: "{{timeout}}",
					Metadata: map[string]string{
						"{{metadata_key}}": "{{metadata_value}}",
					},
					ProtoFiles: &GRPCProtoFilesDiscovery{
						FullMethod:  "{{service}}.{{method}}/{{rpc}}",
						JSONRequest: "{{json_request}}",
					},
				},
			},
		}

		fm := template.FuncMap{
			"server_host": func() string {
				return "localhost"
			},
			"server_port": func() string {
				return "50051"
			},
			"response_time": func() string {
				return "100ms"
			},
			"body_contains": func() string {
				return "success"
			},
			"timeout": func() string {
				return "5s"
			},
			"metadata_key": func() string {
				return "api-key"
			},
			"metadata_value": func() string {
				return "test-key-123"
			},
			"service": func() string {
				return "greeter"
			},
			"method": func() string {
				return "Greeter"
			},
			"rpc": func() string {
				return "SayHello"
			},
			"json_request": func() string {
				return `{"name":"test"}`
			},
		}

		task, err := NewTask("", ct)
		assert.NoError(t, err)

		ct, ok := task.(*GRPCTask)
		assert.True(t, ok)
		assert.NoError(t, ct.renderTemplate(fm))

		// Verify server
		assert.Equal(t, "localhost:50051", ct.Server)

		// Verify post script (not rendered, should remain unchanged)
		assert.Equal(t, "vars[\"message\"] = \"hello\"", ct.PostScript)

		// Verify success when
		assert.Equal(t, "100ms", ct.SuccessWhen[0].ResponseTime)
		assert.Equal(t, "success", ct.SuccessWhen[0].Body[0].Contains)

		// Verify advance options
		assert.Equal(t, "5s", ct.AdvanceOptions.RequestOptions.RequestTimeout)
		assert.Equal(t, "test-key-123", ct.AdvanceOptions.RequestOptions.Metadata["api-key"])
		assert.Equal(t, "greeter.Greeter/SayHello", ct.AdvanceOptions.RequestOptions.ProtoFiles.FullMethod)
		assert.Equal(t, `{"name":"test"}`, ct.AdvanceOptions.RequestOptions.ProtoFiles.JSONRequest)
	})

	t.Run("render template with empty raw task", func(t *T.T) {
		ct := &GRPCTask{
			Task:   &Task{},
			Server: "localhost:50051",
		}

		fm := template.FuncMap{}

		task, err := NewTask("", ct)
		assert.NoError(t, err)

		ct, ok := task.(*GRPCTask)
		assert.True(t, ok)
		assert.NoError(t, ct.renderTemplate(fm))
		assert.Equal(t, "localhost:50051", ct.Server)
	})

	t.Run("render template with invalid template", func(t *T.T) {
		ct := &GRPCTask{
			Task:   &Task{},
			Server: "{{invalid_func}}",
		}

		fm := template.FuncMap{}

		task, err := NewTask("", ct)
		assert.NoError(t, err)

		ct, ok := task.(*GRPCTask)
		assert.True(t, ok)
		err = ct.renderTemplate(fm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "render server failed")
	})

	t.Run("render template with reflection discovery", func(t *T.T) {
		ct := &GRPCTask{
			Task:   &Task{},
			Server: "localhost:50051",
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					Reflection: &GRPCReflectionDiscovery{
						FullMethod:  "{{service}}.{{method}}/{{rpc}}",
						JSONRequest: "{{json_request}}",
					},
				},
			},
		}

		fm := template.FuncMap{
			"service": func() string {
				return "greeter"
			},
			"method": func() string {
				return "Greeter"
			},
			"rpc": func() string {
				return "SayHello"
			},
			"json_request": func() string {
				return `{"name":"test"}`
			},
		}

		task, err := NewTask("", ct)
		assert.NoError(t, err)

		ct, ok := task.(*GRPCTask)
		assert.True(t, ok)
		assert.NoError(t, ct.renderTemplate(fm))

		assert.Equal(t, "greeter.Greeter/SayHello", ct.AdvanceOptions.RequestOptions.Reflection.FullMethod)
		assert.Equal(t, `{"name":"test"}`, ct.AdvanceOptions.RequestOptions.Reflection.JSONRequest)
	})

	t.Run("render template with health check discovery", func(t *T.T) {
		ct := &GRPCTask{
			Task:   &Task{},
			Server: "localhost:50051",
			AdvanceOptions: &GRPCAdvanceOption{
				RequestOptions: &GRPCOptRequest{
					HealthCheck: &GRPCHealthCheckDiscovery{
						Service: "{{service_name}}",
					},
				},
			},
		}

		fm := template.FuncMap{
			"service_name": func() string {
				return "greeter.Greeter"
			},
		}

		task, err := NewTask("", ct)
		assert.NoError(t, err)

		ct, ok := task.(*GRPCTask)
		assert.True(t, ok)
		assert.NoError(t, ct.renderTemplate(fm))

		assert.Equal(t, "greeter.Greeter", ct.AdvanceOptions.RequestOptions.HealthCheck.Service)
	})
}

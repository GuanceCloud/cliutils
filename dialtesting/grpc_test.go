// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	T "testing"
	"time"

	pb "github.com/GuanceCloud/cliutils/dialtesting/greeter"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("recv req from %v", in.GetName())
	return &pb.HelloReply{Message: "hello " + in.GetName()}, nil
}

func TestGRPCDial(t *T.T) {
	lsn, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)

	t.Logf("listen on %s", lsn.Addr().String())
	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &server{})

	healthSrv := health.NewServer()

	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	//  we can set specific service's status
	healthSrv.SetServingStatus("greeter.Greeter", grpc_health_v1.HealthCheckResponse_SERVING)

	grpc_health_v1.RegisterHealthServer(s, healthSrv)
	reflection.Register(s)

	go func() {
		assert.NoError(t, s.Serve(lsn)) // start server
	}()

	time.Sleep(time.Second) // wait

	t.Run(`dial-on-health-check(with-reflection)`, func(t *T.T) {
		task := &GRPCTask{
			Server:     lsn.Addr().String(),
			FullMethod: "greeter.Greeter/SayHello",
		}

		assert.NoError(t, task.init())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		cli := grpc_health_v1.NewHealthClient(task.conn)
		req := &grpc_health_v1.HealthCheckRequest{
			// set service name for specifi service
			Service: "greeter.Greeter",
		}

		resp, err := cli.Check(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.GetStatus())
	})

	t.Run(`dial-on-health-check`, func(t *T.T) {
		task := &GRPCTask{
			Server: lsn.Addr().String(),
		}

		assert.NoError(t, task.init())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		cli := grpc_health_v1.NewHealthClient(task.conn)
		req := &grpc_health_v1.HealthCheckRequest{
			// set service name for specifi service
			Service: "greeter.Greeter",
		}

		resp, err := cli.Check(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.GetStatus())
	})

	t.Run(`dial-on-health-check(service-not-exist)`, func(t *T.T) {
		task := &GRPCTask{
			Server: lsn.Addr().String(),
		}

		assert.NoError(t, task.init())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		cli := grpc_health_v1.NewHealthClient(task.conn)
		req := &grpc_health_v1.HealthCheckRequest{
			// the service not exist
			Service: "greeter.SomeServiceNotExist",
		}

		resp, err := cli.Check(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, grpc_health_v1.HealthCheckResponse_UNKNOWN, resp.GetStatus())
	})

	t.Run(`dial-on-proto-file(with-behavior)`, func(t *T.T) {
		proto, err := os.ReadFile("greeter/greeter.proto")
		assert.NoError(t, err)

		hr := &pb.HelloRequest{
			Name: "world",
		}

		j, err := json.Marshal(hr)
		assert.NoError(t, err)

		task := &GRPCTask{
			Server:     lsn.Addr().String(),
			FullMethod: "greeter.Greeter/SayHello",
			ProtoFiles: map[string]string{
				"greeter.proto": string(proto),
			},
			JSONRequest: j,
		}

		assert.NoError(t, task.init())
		assert.NoError(t, task.run())

		defer task.stop()

		t.Logf("result: %s", string(task.result))
	})
}

func TestGRPCTask_Check(t *T.T) {
	t.Run("valid task", func(t *T.T) {
		task := &GRPCTask{
			Server:     "localhost:50051",
			FullMethod: "greeter.Greeter/SayHello",
		}
		task.initTask()
		assert.NoError(t, task.check())
	})

	t.Run("missing server", func(t *T.T) {
		task := &GRPCTask{
			FullMethod: "greeter.Greeter/SayHello",
		}
		task.initTask()
		err := task.check()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server address is required")
	})

	t.Run("missing full method", func(t *T.T) {
		task := &GRPCTask{
			Server: "localhost:50051",
		}
		task.initTask()
		err := task.check()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "full method is required")
	})
}

func TestGRPCTask_Init(t *T.T) {
	serverAddr := "localhost:50051" // Change to your server address

	t.Run("init with reflection", func(t *T.T) {
		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)
		assert.NotNil(t, task.conn)
		assert.NotNil(t, task.method)
		assert.Equal(t, DefaultGRPCTimeout, task.timeout)

		defer task.stop()
	})

	t.Run("init with proto files", func(t *T.T) {
		proto, err := os.ReadFile("greeter/greeter.proto")
		assert.NoError(t, err)

		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
			ProtoFiles: map[string]string{
				"greeter.proto": string(proto),
			},
		}
		task.initTask()

		err = task.init()
		assert.NoError(t, err)
		assert.NotNil(t, task.conn)
		assert.NotNil(t, task.method)

		defer task.stop()
	})

	t.Run("init with custom timeout", func(t *T.T) {
		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
			Timeout:    "10s",
		}
		task.initTask()

		err := task.init()
		assert.NoError(t, err)
		assert.Equal(t, 10*time.Second, task.timeout)

		defer task.stop()
	})

	t.Run("init with invalid timeout", func(t *T.T) {
		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
			Timeout:    "invalid",
		}
		task.initTask()

		err := task.init()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout")
	})

	t.Run("init with invalid server", func(t *T.T) {
		task := &GRPCTask{
			Server:     "invalid:99999",
			FullMethod: "greeter.Greeter/SayHello",
		}
		task.initTask()

		err := task.init()
		assert.Error(t, err)
	})

	t.Run("init with method not found", func(t *T.T) {
		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/NotFoundMethod",
		}
		task.initTask()

		err := task.init()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGRPCTask_Run(t *T.T) {
	serverAddr := "localhost:50051" // Change to your server address

	t.Run("run success", func(t *T.T) {
		greeterProto, err := os.ReadFile("greeter/greeter.proto")
		assert.NoError(t, err)
		userProto, err := os.ReadFile("greeter/user.proto")
		assert.NoError(t, err)

		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHelloToUser",
			ProtoFiles: map[string]string{
				"greeter.proto": string(greeterProto),
				"user.proto":    string(userProto),
			},
		}
		task.initTask()

		requestData := map[string]interface{}{
			"user_id": 1,
		}
		jsonRequest, err := json.Marshal(requestData)
		assert.NoError(t, err)
		task.JSONRequest = jsonRequest

		err = task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)
		assert.NotNil(t, task.result)
		assert.NotEmpty(t, task.result)
		assert.Empty(t, task.reqError)
		assert.Greater(t, task.reqCost, time.Duration(0))

		defer task.stop()
	})

	t.Run("run without init", func(t *T.T) {
		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
		}
		task.initTask()

		err := task.run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "method nil")
	})
}

func TestGRPCTask_GetResults(t *T.T) {
	serverAddr := "localhost:50051" // Change to your server address

	t.Run("success result", func(t *T.T) {
		task := &GRPCTask{
			Task: &Task{
				Name: "test-task",
				Tags: map[string]string{
					"env": "test",
				},
			},
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
		}
		task.initTask()

		requestData := map[string]interface{}{"name": "test"}
		jsonRequest, _ := json.Marshal(requestData)
		task.JSONRequest = jsonRequest

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.NoError(t, err)

		tags, fields := task.getResults()

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
		assert.NotNil(t, fields["response"])
		assert.NotNil(t, fields["message"])

		defer task.stop()
	})

	t.Run("failure result", func(t *T.T) {
		task := &GRPCTask{
			Task: &Task{
				Name: "test-task-fail",
			},
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
		}
		task.initTask()

		// Set invalid request to cause error
		task.JSONRequest = []byte(`invalid json`)

		err := task.init()
		assert.NoError(t, err)

		err = task.run()
		assert.Error(t, err)

		tags, fields := task.getResults()

		// Check tags
		assert.Equal(t, "FAIL", tags["status"])

		// Check fields
		assert.Equal(t, int64(-1), fields["success"])
		assert.NotNil(t, fields["fail_reason"])

		defer task.stop()
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
		t.Run("success", func(t *T.T) {
			task := &GRPCTask{
				result: []byte("test"),
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
		})

		t.Run("no response", func(t *T.T) {
			task := &GRPCTask{}
			reasons, flag := task.checkResult()
			assert.NotEmpty(t, reasons)
			assert.False(t, flag)
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

		t.Run("without port", func(t *T.T) {
			task := &GRPCTask{
				Server: "localhost",
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
		task := &GRPCTask{}
		_, err := task.getVariableValue(Variable{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not support")
	})

	t.Run("getRawTask", func(t *T.T) {
		task := &GRPCTask{
			Server:     "localhost:50051",
			FullMethod: "greeter.Greeter/SayHello",
			Timeout:    "30s",
		}
		task.initTask()

		taskJSON, _ := json.Marshal(task)
		rawTask, err := task.getRawTask(string(taskJSON))
		assert.NoError(t, err)

		var parsed GRPCTask
		err = json.Unmarshal([]byte(rawTask), &parsed)
		assert.NoError(t, err)
		assert.Equal(t, task.Server, parsed.Server)
		assert.Equal(t, task.FullMethod, parsed.FullMethod)
		assert.Equal(t, task.Timeout, parsed.Timeout)
	})
}

func TestGRPCTask_Timeout(t *T.T) {
	serverAddr := "localhost:50051" // Change to your server address

	t.Run("timeout works", func(t *T.T) {
		task := &GRPCTask{
			Server:     serverAddr,
			FullMethod: "greeter.Greeter/SayHello",
			Timeout:    "100ms",
		}
		task.initTask()

		requestData := map[string]interface{}{"name": "test"}
		jsonRequest, _ := json.Marshal(requestData)
		task.JSONRequest = jsonRequest

		err := task.init()
		assert.NoError(t, err)

		// Test that timeout is set correctly
		assert.Equal(t, 100*time.Millisecond, task.timeout)

		err = task.run()
		assert.NoError(t, err) // Should succeed within timeout

		defer task.stop()
	})
}

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

	go func() {
		assert.NoError(t, s.Serve(lsn)) // start server
	}()

	time.Sleep(time.Second) // wait

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

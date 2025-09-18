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
)

type server struct {
	pb.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("recv req from %v", in.GetName())
	return &pb.HelloReply{Message: "hello " + in.GetName()}, nil
}

func TestGRPCDial(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		lsn, err := net.Listen("tcp", ":0")
		assert.NoError(t, err)

		t.Logf("listen on %s", lsn.Addr().String())
		s := grpc.NewServer()
		pb.RegisterGreeterServer(s, &server{})

		go func() {
			assert.NoError(t, s.Serve(lsn)) // start server
		}()

		time.Sleep(time.Second) // wait

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

		t.Logf("result: %s", string(task.result))
	})
}

// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	pdesc "github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type GRPCTask struct {
	*Task
	Server      string            `json:"server"`
	FullMethod  string            `json:"full_method"`
	ProtoFiles  map[string]string `json:"protofiles"` // user's multiple .proto files
	JSONRequest []byte            `json:"request"`    // user's gRPC request are JSON bytes

	conn   *grpc.ClientConn
	method *pdesc.MethodDescriptor

	result []byte
}

func (t *GRPCTask) stop() {
	t.conn.Close()
}

func (t *GRPCTask) init() error {
	conn, err := grpc.Dial(t.Server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	t.conn = conn

	if t.FullMethod != "" {
		if err := t.findMethod(); err != nil {
			return err
		}
	}

	return nil
}

func (t *GRPCTask) findMethod() error {
	if len(t.ProtoFiles) == 0 {
		return t.findMethodByReflection()
	}

	if err := t.findMethodAmongProtofiles(); err != nil {
		log.Printf("findMethodAmongProtofiles: %s", err.Error())

		if err := t.findMethodByReflection(); err != nil {
			return err
		}
	}

	return nil
}

func (t *GRPCTask) findMethodByReflection() error {
	rc := grpcreflect.NewClient(context.Background(), rpb.NewServerReflectionClient(t.conn))

	slash := strings.LastIndex(t.FullMethod, "/")
	if slash == -1 {
		fmt.Errorf("invalid full method name: %s", t.FullMethod)
	}
	serviceName := t.FullMethod[:slash]

	// 使用 reflection client 解析服务
	fd, err := rc.FileContainingSymbol(serviceName)
	if err != nil {
		return err
	}

	sd := fd.FindService(serviceName)
	if sd == nil {
		return fmt.Errorf("service %s not found", serviceName)
	}

	methodName := t.FullMethod[slash+1:]
	md := sd.FindMethodByName(methodName)
	if md == nil {
		return fmt.Errorf("method %s not found in service %s", methodName, serviceName)
	}

	log.Printf("find method %q ok", t.FullMethod)

	t.method = md
	return nil
}

func (t *GRPCTask) findMethodAmongProtofiles() error {
	p := protoparse.Parser{
		Accessor: protoparse.FileContentsFromMap(t.ProtoFiles),
	}

	desc, err := p.ParseFiles(getFileNames(t.ProtoFiles)...)
	if err != nil {
		return err
	}

	sepIdx := strings.LastIndex(t.FullMethod, "/")
	if sepIdx == -1 {
		return fmt.Errorf("invalid FullMethod: %q", t.FullMethod)
	}

	service := t.FullMethod[:sepIdx]
	method := t.FullMethod[sepIdx+1:]

	log.Printf("service: %s, method: %s", service, method)

	//reg := &protoregistry.Files{}
	for _, fd := range desc {
		if sd := fd.FindService(service); sd != nil {
			if md := sd.FindMethodByName(method); md != nil {
				t.method = md
			}
		}
	}

	if t.method == nil {
		return fmt.Errorf("method %s not found among proto files", method)
	}

	return nil
}

func getFileNames(files map[string]string) []string {
	arr := make([]string, 0, len(files))
	for k := range files {
		arr = append(arr, k)
	}
	return arr
}

func (t *GRPCTask) run() error {
	// create dynamic gRPC request
	msg := dynamic.NewMessage(t.method.GetInputType())
	if err := msg.UnmarshalJSON(t.JSONRequest); err != nil {
		return fmt.Errorf("invalid message for method %q: %w", t.method.GetName(), err)
	}

	stub := grpcdynamic.NewStub(t.conn)
	resp, err := stub.InvokeRpc(context.Background(), t.method, msg)
	if err != nil {
		// dialtest failed
		return err
	}

	// dial test message
	if j, err := json.Marshal(resp); err != nil {
		return err
	} else {
		t.result = j
	}

	return nil
}

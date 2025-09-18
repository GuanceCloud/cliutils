// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pdesc "github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCTask struct {
	*Task
	Server      string            `json:"server"`
	FullMethod  string            `json:"full_method"`
	ProtoFiles  map[string][]byte `json:"protofiles"` // user's multiple .proto files
	JSONRequest []byte            `json:"request"`    // user's gRPC request are JSON bytes

	conn   *grpc.ClientConn
	method *pdesc.MethodDescriptor

	result []byte
}

func (t *GRPCTask) stop() {
	if err := t.conn.Close(); err != nil {
		return fmt.Errorf("gRPC connection close: %w", err)
	}
	return nil
}

func (t *GRPCTask) init() error {
	conn, err := grpc.Dial(t.Server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	t.conn = conn

	if err := t.findMethod(); err != nil {
		return err
	}

	return nil
}

func (t *GRPCTask) findMethod() error {
	if len(t.ProtoFiles) == 0 {
		return findMethodByReflection()
	}

	if err := t.findMethodAmongProtofiles(); err != nil {
		if err := t.findMethodByReflection(); err != nil {
			return err
		}
	}

	return nil
}

func (t *GRPCTask) findMethodByReflection() error {
	// TODO
	return fmt.Errorf("TODO")
}

func (t *GRPCTask) findMethodAmongProtofiles() error {
	p := protoparse.Parser{
		Accessor: protoparse.FileContentsFromMap(t.ProtoFiles),
	}

	desc, err := p.ParseFiles(getFileNames(t.ProtoFiles)...)
	if err != nil {
		return err
	}

	sepIdx := strings.LastIndex(t.FullMethod, ".")
	if sepIdx == -1 {
		return fmt.Errorf("invalid FullMethod: %q", t.FullMethod)
	}

	service := t.FullMethod[:sepIdx]
	method := t.FullMethod[sepIdx+1:]

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
}

func getFileNames(files map[string][]byte) []string {
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
	resp, err := stub.InvokeRpc(context.Background(), t.method, req)
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

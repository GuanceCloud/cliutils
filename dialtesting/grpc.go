// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	pdesc "github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	_ TaskChild = (*GRPCTask)(nil)
	_ ITask     = (*GRPCTask)(nil)
)

const (
	DefaultGRPCTimeout = 30 * time.Second
)

type GRPCTask struct {
	*Task
	Server      string            `json:"server"`
	FullMethod  string            `json:"full_method"`
	ProtoFiles  map[string]string `json:"protofiles"` // user's multiple .proto files
	JSONRequest []byte            `json:"request"`    // user's gRPC request are JSON bytes
	Timeout     string            `json:"timeout"`    // request timeout, e.g., "30s", "1m"

	conn   *grpc.ClientConn
	method *pdesc.MethodDescriptor

	result   []byte
	reqError string
	reqCost  time.Duration
	timeout  time.Duration
}

func (t *GRPCTask) stop() {
	if t.conn != nil {
		t.conn.Close()
	}
}

func (t *GRPCTask) init() error {
	// parse timeout
	t.timeout = DefaultGRPCTimeout
	if t.Timeout != "" {
		timeout, err := time.ParseDuration(t.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", t.Timeout, err)
		}
		t.timeout = timeout
	}

	conn, err := grpc.Dial(t.Server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	t.conn = conn

	if t.FullMethod != "" {
		ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
		defer cancel()
		if err := t.findMethod(ctx); err != nil {
			t.conn.Close()
			return err
		}
	}

	return nil
}

func (t *GRPCTask) findMethod(ctx context.Context) error {
	if len(t.ProtoFiles) > 0 {
		err := t.findMethodAmongProtofiles()
		if err != nil {
			return fmt.Errorf("find method via proto files: %w", err)
		}
		return nil
	}

	err := t.findMethodByReflection(ctx)
	if err != nil {
		return fmt.Errorf("find method via reflection: %w", err)
	}
	return nil
}

func (t *GRPCTask) findMethodByReflection(ctx context.Context) error {
	rc := grpcreflect.NewClientAuto(ctx, t.conn)
	defer rc.Reset()

	slash := strings.LastIndex(t.FullMethod, "/")
	if slash == -1 {
		return fmt.Errorf("invalid full method name: %s", t.FullMethod)
	}
	serviceName := t.FullMethod[:slash]

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
	t.method = md
	return nil
}

func (t *GRPCTask) findMethodAmongProtofiles() error {
	extendedMap := buildExtendedProtoMap(t.ProtoFiles)

	p := protoparse.Parser{
		Accessor:         protoparse.FileContentsFromMap(extendedMap),
		InferImportPaths: true,
	}

	desc, err := p.ParseFiles(getFileNames(t.ProtoFiles)...)
	if err != nil {
		return fmt.Errorf("parse proto files failed: %w", err)
	}

	sepIdx := strings.LastIndex(t.FullMethod, "/")
	if sepIdx == -1 {
		return fmt.Errorf("invalid fullMethod: %q", t.FullMethod)
	}

	service := t.FullMethod[:sepIdx]
	method := t.FullMethod[sepIdx+1:]
	for _, fd := range desc {
		if sd := fd.FindService(service); sd != nil {
			if md := sd.FindMethodByName(method); md != nil {
				t.method = md
				return nil
			}
		}
	}

	return fmt.Errorf("method %s not found in service %s", t.FullMethod, service)
}

func buildExtendedProtoMap(protoFiles map[string]string) map[string]string {
	extendedMap := make(map[string]string)

	// Add original files and their base names
	for k, v := range protoFiles {
		extendedMap[k] = v
		extendedMap[filepath.Base(k)] = v
	}

	// Parse imports and build mappings: for each import, find matching file by base name
	for _, content := range protoFiles {
		for _, imp := range extractImports(content) {
			if extendedMap[imp] == "" {
				importBase := filepath.Base(imp)
				for filename, fileContent := range protoFiles {
					if filepath.Base(filename) == importBase {
						extendedMap[imp] = fileContent
						break
					}
				}
			}
		}
	}

	return extendedMap
}

// extractImports extracts all import statements from proto file content
func extractImports(content string) []string {
	var imports []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "import ") {
			start := strings.Index(line, `"`)
			if start != -1 {
				end := strings.Index(line[start+1:], `"`)
				if end != -1 {
					imports = append(imports, line[start+1:start+1+end])
				}
			}
		}
	}
	return imports
}

func getFileNames(files map[string]string) []string {
	arr := make([]string, 0, len(files))
	for k := range files {
		arr = append(arr, k)
	}
	return arr
}

func (t *GRPCTask) run() error {
	if t.method == nil {
		t.reqError = "method not initialized"
		return fmt.Errorf("method nil")
	}

	start := time.Now()
	defer func() {
		t.reqCost = time.Since(start)
	}()

	// create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()

	// create dynamic gRPC request
	msg := dynamic.NewMessage(t.method.GetInputType())
	if err := msg.UnmarshalJSON(t.JSONRequest); err != nil {
		t.reqError = fmt.Sprintf("invalid message: %v", err)
		return fmt.Errorf("invalid message for method %q: %w", t.method.GetName(), err)
	}

	stub := grpcdynamic.NewStub(t.conn)
	resp, err := stub.InvokeRpc(ctx, t.method, msg)
	if err != nil {
		t.reqError = err.Error()
		return err
	}

	// dial test message
	j, err := json.Marshal(resp)
	if err != nil {
		t.reqError = fmt.Sprintf("marshal response failed: %v", err)
		return err
	}
	t.result = j

	return nil
}

func (t *GRPCTask) class() string {
	return ClassGRPC
}

func (t *GRPCTask) metricName() string {
	return "grpc_dial_testing"
}

func (t *GRPCTask) initTask() {
	if t.Task == nil {
		t.Task = &Task{}
	}
}

func (t *GRPCTask) check() error {
	if t.Server == "" {
		return fmt.Errorf("server address is required")
	}
	if t.FullMethod == "" {
		return fmt.Errorf("full method is required")
	}
	return nil
}

func (t *GRPCTask) clear() {
	t.result = nil
	t.reqError = ""
	t.reqCost = 0
	if t.timeout == 0 {
		t.timeout = DefaultGRPCTimeout
	}
}

func (t *GRPCTask) checkResult() ([]string, bool) {
	if t.reqError != "" {
		return []string{t.reqError}, false
	}
	if t.result == nil {
		return []string{"no response"}, false
	}
	return nil, true
}

func (t *GRPCTask) getResults() (tags map[string]string, fields map[string]interface{}) {
	tags = map[string]string{
		"name":   t.Name,
		"server": t.Server,
		"method": t.FullMethod,
		"status": "FAIL",
		"proto":  "grpc",
	}

	fields = map[string]interface{}{
		"response_time": int64(t.reqCost) / 1000,
		"success":       int64(-1),
	}

	if hostnames, err := t.getHostName(); err == nil && len(hostnames) > 0 {
		tags["dest_host"] = hostnames[0]
	}

	for k, v := range t.Tags {
		tags[k] = v
	}

	message := map[string]interface{}{}

	reasons, succFlag := t.checkResult()
	if t.reqError != "" {
		reasons = append(reasons, t.reqError)
	}

	if succFlag && t.reqError == "" {
		tags["status"] = "OK"
		fields["success"] = int64(1)
		message["response_time"] = int64(t.reqCost) / 1000
		if t.result != nil {
			message["response"] = string(t.result)
		}
	} else {
		message["fail_reason"] = strings.Join(reasons, ";")
		fields["fail_reason"] = strings.Join(reasons, ";")
	}

	if t.result != nil {
		fields["response"] = string(t.result)
	}

	data, err := json.Marshal(message)
	if err != nil {
		fields["message"] = err.Error()
	} else {
		if len(data) > MaxMsgSize {
			fields["message"] = string(data[:MaxMsgSize])
		} else {
			fields["message"] = string(data)
		}
	}

	return tags, fields
}

func (t *GRPCTask) beforeFirstRender() {
}

func (t *GRPCTask) getVariableValue(variable Variable) (string, error) {
	return "", fmt.Errorf("gRPC dial test does not support variable extraction")
}

func (t *GRPCTask) getHostName() ([]string, error) {
	if t.Server == "" {
		return nil, fmt.Errorf("server address is empty")
	}

	host, _, err := net.SplitHostPort(t.Server)
	if err == nil {
		return []string{host}, nil
	}

	return []string{t.Server}, nil
}

func (t *GRPCTask) getRawTask(taskString string) (string, error) {
	task := GRPCTask{}

	if err := json.Unmarshal([]byte(taskString), &task); err != nil {
		return "", fmt.Errorf("unmarshal grpc task failed: %w", err)
	}

	task.Task = nil

	bytes, _ := json.Marshal(task)
	return string(bytes), nil
}

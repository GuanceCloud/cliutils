// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	_ TaskChild = (*BrowserTask)(nil)
	_ ITask     = (*BrowserTask)(nil)
)

const (
	defaultBrowserDialPath    = "browser-dial"
	defaultBrowserDialTimeout = 30_000

	optionBrowserDialPath      = "browser_dial_path"
	optionBrowserDialPathCamel = "browserDialPath"
	optionLightpandaPath       = "lightpanda_path"
	optionLightpandaPathCamel  = "lightpandaPath"
)

type BrowserTask struct {
	*Task
	BrowserConfig string `json:"browser_config"`

	duration time.Duration
	result   browserDialRun
	exitCode int
	reqError string
	stderr   string
	rawTask  *BrowserTask

	cancelMu sync.Mutex
	cancel   *browserTaskCancel
}

type browserTaskCancel struct {
	cancel context.CancelFunc
}

type browserConfig struct {
	Name       string              `yaml:"name"`
	Target     string              `yaml:"target"`
	TimeoutMS  int                 `yaml:"timeout_ms"`
	Tags       map[string]string   `yaml:"tags"`
	ConfigVars []ConfigVar         `yaml:"config_vars"`
	Steps      []browserConfigStep `yaml:"steps"`
}

type browserConfigStep struct {
	Action string `yaml:"action"`
	URL    string `yaml:"url"`
}

type browserDialOutput struct {
	ExitCode int            `json:"exit_code"`
	Run      browserDialRun `json:"run"`
}

type browserDialRun struct {
	RunID      string            `json:"run_id"`
	Name       string            `json:"name"`
	Target     string            `json:"target,omitempty"`
	Status     string            `json:"status"`
	Success    bool              `json:"success"`
	DurationUS int64             `json:"duration_us"`
	Steps      []browserDialStep `json:"steps"`
	TraceIDs   []string          `json:"trace_ids,omitempty"`
	Error      *browserDialError `json:"error,omitempty"`
	FailReason string            `json:"fail_reason,omitempty"`
}

type browserDialStep struct {
	Seq        int               `json:"seq"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	DurationUS int64             `json:"duration_us"`
	URL        string            `json:"url,omitempty"`
	Title      string            `json:"title,omitempty"`
	Error      *browserDialError `json:"error,omitempty"`
}

type browserDialError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
}

func (t *BrowserTask) clear() {
	t.duration = 0
	t.result = browserDialRun{}
	t.exitCode = 0
	t.reqError = ""
	t.stderr = ""
}

func (t *BrowserTask) stop() {
	t.cancelMu.Lock()
	cancel := t.cancel
	t.cancelMu.Unlock()
	if cancel != nil && cancel.cancel != nil {
		cancel.cancel()
	}
}

func (t *BrowserTask) class() string {
	return ClassHeadless
}

func (t *BrowserTask) metricName() string {
	return "browser_dial_testing"
}

func (t *BrowserTask) run() error {
	start := time.Now()
	path, err := t.writeScriptFile()
	if err != nil {
		t.reqError = err.Error()
		return nil
	}
	defer os.Remove(path) //nolint:errcheck

	timeoutMS := t.effectiveTimeoutMS()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMS)*time.Millisecond+15*time.Second)
	defer cancel()
	cancelState := t.setCancel(cancel)
	defer t.clearCancel(cancelState)

	args := []string{
		"run", path,
		"--dry-run",
		"--skip-token-check",
		"--json",
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmdArgs := executableArgs(args)
	cmd := exec.CommandContext(ctx, t.executablePath(), cmdArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if lightpandaPath := t.lightpandaPath(); lightpandaPath != "" {
		cmd.Env = append(os.Environ(), "LIGHTPANDA_EXECUTABLE_PATH="+lightpandaPath)
	}

	err = cmd.Run()
	t.duration = time.Since(start)
	t.stderr = strings.TrimSpace(stderr.String())
	if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
		t.exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.reqError = err.Error()
		return nil
	}

	var output browserDialOutput
	if decodeErr := json.Unmarshal(stdout.Bytes(), &output); decodeErr != nil {
		t.reqError = fmt.Sprintf("parse browser-dial output failed: %s", decodeErr)
		if t.stderr != "" {
			t.reqError += ": " + t.stderr
		}
		return nil
	}
	t.exitCode = output.ExitCode
	t.result = output.Run
	return nil
}

func (t *BrowserTask) setCancel(cancel context.CancelFunc) *browserTaskCancel {
	cancelState := &browserTaskCancel{cancel: cancel}
	t.cancelMu.Lock()
	t.cancel = cancelState
	t.cancelMu.Unlock()
	return cancelState
}

func (t *BrowserTask) clearCancel(cancelState *browserTaskCancel) {
	t.cancelMu.Lock()
	if t.cancel == cancelState {
		t.cancel = nil
	}
	t.cancelMu.Unlock()
}

func executableArgs(args []string) []string {
	if os.Getenv("GO_WANT_BROWSER_DIAL_HELPER") == "" {
		return args
	}
	return append([]string{"-test.run=TestBrowserDialHelperProcess", "--"}, args...)
}

func (t *BrowserTask) writeScriptFile() (string, error) {
	file, err := os.CreateTemp("", "dialtesting-browser-*.yaml")
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck

	if _, err := file.WriteString(t.BrowserConfig); err != nil {
		os.Remove(file.Name()) //nolint:errcheck
		return "", err
	}
	return file.Name(), nil
}

func (t *BrowserTask) executablePath() string {
	if value := t.GetOption()[optionBrowserDialPath]; value != "" {
		return value
	}
	if value := t.GetOption()[optionBrowserDialPathCamel]; value != "" {
		return value
	}
	if value := os.Getenv("BROWSER_DIAL_PATH"); value != "" {
		return value
	}
	return defaultBrowserDialPath
}

func (t *BrowserTask) lightpandaPath() string {
	if value := t.GetOption()[optionLightpandaPath]; value != "" {
		return value
	}
	if value := t.GetOption()[optionLightpandaPathCamel]; value != "" {
		return value
	}
	return ""
}

func (t *BrowserTask) effectiveTimeoutMS() int {
	cfg, err := t.parseBrowserConfig()
	if err == nil && cfg.TimeoutMS > 0 {
		return cfg.TimeoutMS
	}
	return defaultBrowserDialTimeout
}

func (t *BrowserTask) checkResult() (reasons []string, succFlag bool) {
	if t.reqError != "" {
		return []string{t.reqError}, false
	}
	if t.result.Success {
		return nil, true
	}
	if t.result.FailReason != "" {
		reasons = append(reasons, t.result.FailReason)
	}
	if t.result.Error != nil && t.result.Error.Message != "" {
		reasons = append(reasons, t.result.Error.Message)
	}
	if len(reasons) == 0 && t.stderr != "" {
		reasons = append(reasons, t.stderr)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "browser dial failed")
	}
	return reasons, false
}

func (t *BrowserTask) getResults() (tags map[string]string, fields map[string]interface{}) {
	cfg, _ := t.parseBrowserConfig()
	name := firstNonEmpty(t.result.Name, cfg.Name, t.Name)
	target := firstNonEmpty(t.result.Target, cfg.Target)
	tags = map[string]string{
		"name":           name,
		"url":            target,
		"status":         "FAIL",
		"runner":         "browser-dial",
		"browser_engine": "lightpanda",
	}
	for k, v := range cfg.Tags {
		tags[k] = v
	}
	for k, v := range t.Tags {
		tags[k] = v
	}

	responseTime := t.result.DurationUS
	if responseTime == 0 {
		responseTime = int64(t.duration) / 1000
	}
	fields = map[string]interface{}{
		"response_time":  responseTime,
		"success":        int64(-1),
		"last_step":      int64(lastBrowserStep(t.result.Steps)),
		"browser_run_id": t.result.RunID,
		"exit_code":      int64(t.exitCode),
	}

	if t.reqError == "" && t.result.Success {
		tags["status"] = "OK"
		fields["success"] = int64(1)
		fields["message"] = "success"
	} else {
		reasons, _ := t.checkResult()
		fields["fail_reason"] = strings.Join(reasons, ";")
		fields["message"] = strings.Join(reasons, ";")
	}
	if len(t.result.Steps) > 0 {
		last := t.result.Steps[len(t.result.Steps)-1]
		fields["page_url"] = last.URL
		fields["page_title"] = last.Title
	}
	if len(t.result.TraceIDs) > 0 {
		fields["trace_id"] = t.result.TraceIDs[0]
		if traceIDs, err := json.Marshal(t.result.TraceIDs); err == nil {
			fields["trace_ids"] = string(traceIDs)
		}
	}
	if steps, err := json.Marshal(t.result.Steps); err == nil {
		fields["steps"] = string(steps)
	}

	return tags, fields
}

func lastBrowserStep(steps []browserDialStep) int {
	if len(steps) == 0 {
		return 0
	}
	return steps[len(steps)-1].Seq
}

func (t *BrowserTask) check() error {
	if strings.TrimSpace(t.BrowserConfig) == "" {
		return errors.New("browser_config should not be empty")
	}
	cfg, err := t.parseBrowserConfig()
	if err != nil {
		return fmt.Errorf("parse browser_config failed: %w", err)
	}
	if len(cfg.Steps) == 0 {
		return errors.New("browser_config steps should not be empty")
	}
	return nil
}

func (t *BrowserTask) init() error {
	return nil
}

func (t *BrowserTask) getHostName() ([]string, error) {
	cfg, err := t.parseBrowserConfig()
	if err != nil {
		return nil, err
	}
	hosts := make([]string, 0, 1)
	if strings.TrimSpace(cfg.Target) != "" {
		host, err := getHostName(cfg.Target)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	for _, step := range cfg.Steps {
		if step.Action != "goto" || strings.TrimSpace(step.URL) == "" {
			continue
		}
		host, err := getHostName(step.URL)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	if len(hosts) == 0 {
		return nil, errors.New("browser_config target or goto url should not be empty")
	}
	return dedupBrowserHostNames(hosts), nil
}

func (t *BrowserTask) getVariableValue(variable Variable) (string, error) {
	return "", errors.New("not support")
}

func (t *BrowserTask) getRawTask(taskString string) (string, error) {
	task := BrowserTask{}
	if err := json.Unmarshal([]byte(taskString), &task); err != nil {
		return "", fmt.Errorf("unmarshal browser task failed: %w", err)
	}
	task.Task = nil
	task.BrowserConfig = sanitizeBrowserConfig(task.BrowserConfig)
	bytes, _ := json.Marshal(task)
	return string(bytes), nil
}

func (t *BrowserTask) renderTemplate(fm template.FuncMap) error {
	if t.rawTask == nil {
		task := &BrowserTask{}
		if err := t.NewRawTask(task); err != nil {
			return fmt.Errorf("new raw task failed: %w", err)
		}
		t.rawTask = task
	}
	if t.rawTask == nil {
		return errors.New("raw task is nil")
	}

	browserConfig, err := t.GetParsedString(t.rawTask.BrowserConfig, fm)
	if err != nil {
		return fmt.Errorf("render browser_config failed: %w", err)
	}
	t.BrowserConfig = browserConfig
	return nil
}

func (t *BrowserTask) initTask() {
	if t.Task == nil {
		t.Task = &Task{}
	}
}

func (t *BrowserTask) setReqError(err string) {
	t.reqError = err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func dedupBrowserHostNames(hosts []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func (t *BrowserTask) parseBrowserConfig() (browserConfig, error) {
	var cfg browserConfig
	if strings.TrimSpace(t.BrowserConfig) == "" {
		return cfg, errors.New("browser_config is empty")
	}
	if err := yaml.Unmarshal([]byte(t.BrowserConfig), &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func sanitizeBrowserConfig(config string) string {
	if strings.TrimSpace(config) == "" {
		return config
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(config), &node); err != nil {
		return config
	}
	sanitizeYAMLConfigVars(&node)
	data, err := yaml.Marshal(&node)
	if err != nil {
		return config
	}
	return string(data)
}

func sanitizeYAMLConfigVars(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Value == "config_vars" {
				sanitizeConfigVarSequence(value)
				continue
			}
			sanitizeYAMLConfigVars(value)
		}
		return
	}
	for _, child := range node.Content {
		sanitizeYAMLConfigVars(child)
	}
}

func sanitizeConfigVarSequence(node *yaml.Node) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range node.Content {
		if item == nil || item.Kind != yaml.MappingNode {
			continue
		}
		if yamlMapBoolValue(item, "secure") {
			setYAMLMapValue(item, "value", "")
		}
	}
}

func yamlMapBoolValue(node *yaml.Node, key string) bool {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return strings.EqualFold(node.Content[i+1].Value, "true")
		}
	}
	return false
}

func setYAMLMapValue(node *yaml.Node, key string, value string) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Kind = yaml.ScalarNode
			node.Content[i+1].Tag = "!!str"
			node.Content[i+1].Value = value
			return
		}
	}
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

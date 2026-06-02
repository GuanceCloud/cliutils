package script

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.yaml")
	content := `
name: homepage
target: https://example.com
post_url: https://openway.example.com?token=tkn_x
tags:
  owner: platform
metadata:
  suite: smoke
steps:
  - name: open
    action: goto
  - action: assert_title
    contains: Example
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "homepage" || got.Target != "https://example.com" {
		t.Fatalf("unexpected script: %#v", got)
	}
	if got.PostURL != "https://openway.example.com?token=tkn_x" {
		t.Fatalf("unexpected post_url: %q", got.PostURL)
	}
	if got.Tags["owner"] != "platform" {
		t.Fatalf("unexpected tags: %#v", got.Tags)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(got.Steps))
	}
}

func TestValidateRejectsMissingSelector(t *testing.T) {
	err := Script{Steps: []Step{{Action: "click"}}}.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadAuthAndConfigVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.yaml")
	content := `
name: auth-homepage
target: https://example.com/dashboard
auth:
  mode: form
  steps:
    - action: goto
      url: https://example.com/login
    - action: fill
      selector: input[name="password"]
      value_from: LOGIN_PASSWORD
      sensitive: true
config_vars:
  - name: LOGIN_PASSWORD
    value: secret
    secure: true
steps:
  - action: goto
  - action: assert_title
    contains: Example
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Auth.Mode != "form" || len(got.Auth.Steps) != 2 {
		t.Fatalf("unexpected auth: %#v", got.Auth)
	}
	if len(got.ConfigVars) != 1 || !got.ConfigVars[0].Secure {
		t.Fatalf("unexpected config vars: %#v", got.ConfigVars)
	}
}

func TestValidateRejectsSensitiveInlineValue(t *testing.T) {
	err := Script{
		Steps: []Step{
			{Action: "fill", Selector: `input[name="password"]`, Value: "secret", Sensitive: true},
		},
	}.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadRequestEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.yaml")
	content := `
name: request-env
target: https://example.com
headers:
  X-Env: prod
cookies:
  - name: sid
    value_from: SESSION_ID
    domain: example.com
    path: /
    secure: true
    http_only: true
    same_site: Lax
ignore_https_errors: true
proxy_url: http://127.0.0.1:7897
config_vars:
  - name: SESSION_ID
    value: secret
    secure: true
steps:
  - action: goto
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Headers["X-Env"] != "prod" || len(got.Cookies) != 1 || !got.IgnoreHTTPSErrors || got.ProxyURL == "" {
		t.Fatalf("unexpected request environment: %#v", got)
	}
	if got.Cookies[0].ValueFrom != "SESSION_ID" || got.Cookies[0].SameSite != "Lax" {
		t.Fatalf("unexpected cookie: %#v", got.Cookies[0])
	}
}

func TestValidateRejectsInvalidCookie(t *testing.T) {
	err := Script{
		Cookies: []Cookie{{Name: "sid", Value: "inline", ValueFrom: "SESSION_ID"}},
		Steps:   []Step{{Action: "goto", URL: "https://example.com"}},
	}.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	err = Script{
		Cookies: []Cookie{{Name: "sid", SameSite: "Maybe"}},
		Steps:   []Step{{Action: "goto", URL: "https://example.com"}},
	}.Validate()
	if err == nil {
		t.Fatal("expected same_site validation error")
	}
}

func TestValidateCoversErrorBranchesAndStepHelpers(t *testing.T) {
	if got := (Step{Name: "Custom", Action: "goto"}).StepName(3); got != "Custom" {
		t.Fatalf("unexpected step name %q", got)
	}
	if got := (Step{Action: "click"}).StepName(3); got != "03 click" {
		t.Fatalf("unexpected fallback step name %q", got)
	}
	if mode, expected := Expected(Step{Equals: "A", Contains: "B"}); mode != "equals" || expected != "A" {
		t.Fatalf("unexpected expected equals: %s %s", mode, expected)
	}

	cases := []Script{
		{ConfigVars: []ConfigVar{{Name: ""}}, Steps: []Step{{Action: "goto", URL: "https://example.com"}}},
		{ConfigVars: []ConfigVar{{Name: "A"}, {Name: " A "}}, Steps: []Step{{Action: "goto", URL: "https://example.com"}}},
		{Headers: map[string]string{" ": "bad"}, Steps: []Step{{Action: "goto", URL: "https://example.com"}}},
		{Cookies: []Cookie{{Name: ""}}, Steps: []Step{{Action: "goto", URL: "https://example.com"}}},
		{Auth: Auth{Steps: []Step{{Action: "goto", URL: "https://example.com"}}}, Steps: []Step{{Action: "goto", URL: "https://example.com"}}},
		{Auth: Auth{Mode: "form"}, Steps: []Step{{Action: "goto", URL: "https://example.com"}}},
		{Auth: Auth{Mode: "oauth"}, Steps: []Step{{Action: "goto", URL: "https://example.com"}}},
		{Steps: nil},
		{Steps: []Step{{}}},
		{Steps: []Step{{Action: "unknown"}}},
		{Steps: []Step{{Action: "goto"}}},
		{Steps: []Step{{Action: "wait_for_selector"}}},
		{Steps: []Step{{Action: "assert_text", Selector: "body"}}},
		{Steps: []Step{{Action: "assert_title"}}},
		{Steps: []Step{{Action: "assert_url"}}},
		{Steps: []Step{{Action: "eval"}}},
	}
	for _, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}

	jsonPath := filepath.Join(t.TempDir(), "script.json")
	if err := os.WriteFile(jsonPath, []byte(`{"steps":[{"action":"goto","url":"https://example.com"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(jsonPath); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected missing file load error")
	}
	badJSON := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badJSON, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(badJSON); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

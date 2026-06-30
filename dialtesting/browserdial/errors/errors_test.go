package errorsx

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorTypesAndFailureReason(t *testing.T) {
	timeout := TimeoutError{TimeoutMS: 123}
	if !strings.Contains(timeout.Error(), "123ms") {
		t.Fatalf("unexpected timeout error: %s", timeout.Error())
	}
	load := ScriptLoadError{Message: "bad script"}
	if load.Error() != "bad script" {
		t.Fatalf("unexpected script load error: %s", load.Error())
	}
	if got := (AuthError{}).Error(); got != "authentication failed" {
		t.Fatalf("unexpected empty auth error: %s", got)
	}
	root := errors.New("no login")
	auth := AuthError{Err: root}
	if auth.Error() != "no login" || !errors.Is(auth.Unwrap(), root) {
		t.Fatalf("unexpected auth unwrap: %s", auth.Error())
	}

	cases := []struct {
		err           error
		hasFailedStep bool
		want          string
	}{
		{nil, false, ""},
		{auth, false, "auth_error"},
		{timeout, false, "timeout"},
		{load, false, "script_load_error"},
		{errors.New("step"), true, "step_error"},
		{errors.New("script"), false, "script_error"},
	}
	for _, tc := range cases {
		if got := FailureReason(tc.err, tc.hasFailedStep); got != tc.want {
			t.Fatalf("FailureReason(%T, %v) = %q, want %q", tc.err, tc.hasFailedStep, got, tc.want)
		}
	}
	info := ErrorInfo(root)
	if info == nil || !strings.Contains(info.Name, "errorString") || info.Message != "no login" || info.Stack == "" {
		t.Fatalf("unexpected error info: %#v", info)
	}
	if ErrorInfo(nil) != nil {
		t.Fatal("nil error should return nil info")
	}
}

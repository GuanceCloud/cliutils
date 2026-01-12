// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"errors"
	"strings"
	"testing"
)

func TestCacheError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *CacheError
		contains []string
	}{
		{
			name: "basic error",
			err: &CacheError{
				Operation: OpPut,
				Path:      "/tmp/cache",
				Err:       errors.New("disk full"),
			},
			contains: []string{"Put", "path=/tmp/cache", "disk full"},
		},
		{
			name: "error with file and details",
			err: &CacheError{
				Operation: OpRead,
				Path:      "/tmp/cache",
				File:      "data.0001",
				Details:   "header corrupted",
				Err:       errors.New("bad header"),
			},
			contains: []string{"Read", "path=/tmp/cache", "file=data.0001", "header corrupted", "bad header"},
		},
		{
			name: "error with caller",
			err: &CacheError{
				Operation: OpRotate,
				Path:      "/tmp/cache",
				Err:       errors.New("permission denied"),
				Caller:    "rotate.go:123",
			},
			contains: []string{"Rotate", "path=/tmp/cache", "permission denied", "caller: rotate.go:123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			for _, contain := range tt.contains {
				if !strings.Contains(errStr, contain) {
					t.Errorf("Error string should contain %q, got: %s", contain, errStr)
				}
			}
		})
	}
}

func TestCacheError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	cacheErr := &CacheError{
		Operation: OpPut,
		Err:       originalErr,
	}

	if !errors.Is(cacheErr, originalErr) {
		t.Error("CacheError should unwrap to original error")
	}
}

func TestNewCacheError(t *testing.T) {
	err := errors.New("test error")
	cacheErr := NewCacheError(OpGet, err, "test details")

	if cacheErr.Operation != OpGet {
		t.Errorf("Expected operation %v, got %v", OpGet, cacheErr.Operation)
	}
	if cacheErr.Err != err {
		t.Errorf("Expected error %v, got %v", err, cacheErr.Err)
	}
	if cacheErr.Details != "test details" {
		t.Errorf("Expected details %q, got %q", "test details", cacheErr.Details)
	}
	if cacheErr.Caller == "" {
		t.Error("Caller should be set automatically")
	}
}

func TestCacheError_WithMethods(t *testing.T) {
	err := errors.New("test error")
	cacheErr := NewCacheError(OpPut, err, "")

	// Test WithPath
	cacheErr = cacheErr.WithPath("/test/path")
	if cacheErr.Path != "/test/path" {
		t.Errorf("Expected path %q, got %q", "/test/path", cacheErr.Path)
	}

	// Test WithFile
	cacheErr = cacheErr.WithFile("test.data")
	if cacheErr.File != "test.data" {
		t.Errorf("Expected file %q, got %q", "test.data", cacheErr.File)
	}

	// Test WithDetails
	cacheErr = cacheErr.WithDetails("additional info")
	if cacheErr.Details != "additional info" {
		t.Errorf("Expected details %q, got %q", "additional info", cacheErr.Details)
	}

	// Test WithDetails with existing details
	cacheErr = cacheErr.WithDetails("more info")
	expected := "additional info: more info"
	if cacheErr.Details != expected {
		t.Errorf("Expected details %q, got %q", expected, cacheErr.Details)
	}
}

func TestWrapFunctions(t *testing.T) {
	baseErr := errors.New("base error")

	t.Run("WrapPutError", func(t *testing.T) {
		err := WrapPutError(baseErr, "/path", 1024)
		if err.Operation != OpPut {
			t.Error("Operation should be Put")
		}
		if err.Path != "/path" {
			t.Error("Path should be set")
		}
		if !strings.Contains(err.Details, "1024") {
			t.Error("Details should contain data size")
		}
	})

	t.Run("WrapGetError", func(t *testing.T) {
		err := WrapGetError(baseErr, "/path", "file.dat")
		if err.Operation != OpGet {
			t.Error("Operation should be Get")
		}
		if err.File != "file.dat" {
			t.Error("File should be set")
		}
	})

	t.Run("WrapRotateError", func(t *testing.T) {
		err := WrapRotateError(baseErr, "/path", "old", "new")
		if err.Operation != OpRotate {
			t.Error("Operation should be Rotate")
		}
		if !strings.Contains(err.Details, "old=old") {
			t.Error("Details should contain old file")
		}
	})
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"temporary error", &CacheError{Operation: OpWrite, Err: errors.New("resource temporarily unavailable")}, true},
		{"permission denied", &CacheError{Operation: OpWrite, Err: errors.New("permission denied")}, false},
		{"lock error", &CacheError{Operation: OpLock, Err: errors.New("locked by alive 1234")}, true},
		{"non-cache error temporary", errors.New("connection refused"), true},
		{"non-cache error permanent", errors.New("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.expected {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestGetErrorContext(t *testing.T) {
	t.Run("cache error", func(t *testing.T) {
		originalErr := errors.New("test error")
		cacheErr := NewCacheError(OpPut, originalErr, "details").
			WithPath("/test").
			WithFile("test.dat")

		context := GetErrorContext(cacheErr)

		if context["operation"] != string(OpPut) {
			t.Error("Operation should be in context")
		}
		if context["path"] != "/test" {
			t.Error("Path should be in context")
		}
		if context["file"] != "test.dat" {
			t.Error("File should be in context")
		}
		if context["details"] != "details" {
			t.Error("Details should be in context")
		}
		if context["original_error"] != "test error" {
			t.Error("Original error should be in context")
		}
		if context["retryable"] != false {
			t.Error("Retryable should be false for test error")
		}
	})

	t.Run("regular error", func(t *testing.T) {
		err := errors.New("connection refused")
		context := GetErrorContext(err)

		if context["original_error"] != "connection refused" {
			t.Error("Original error should be in context")
		}
		if context["retryable"] != true {
			t.Error("Connection refused should be retryable")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		context := GetErrorContext(nil)
		if len(context) != 0 {
			t.Error("Context should be empty for nil error")
		}
	})
}

func TestErrorIntegration(t *testing.T) {
	// This test demonstrates how the enhanced errors work in practice
	t.Run("error chaining example", func(t *testing.T) {
		// Simulate a disk full error during Put with no drop policy
		cache, err := Open(WithPath(t.TempDir()), WithCapacity(100), WithNoDrop(true)) // Very small capacity, no drop
		if err != nil {
			t.Fatalf("Failed to open cache: %v", err)
		}
		defer cache.Close()

		// Try to put data that exceeds capacity
		data := make([]byte, 200) // Larger than capacity
		err = cache.Put(data)

		// Check that we get a wrapped error
		if err == nil {
			t.Fatal("Expected error when exceeding capacity with no-drop policy")
		}

		// Check error structure
		var cacheErr *CacheError
		if !isCacheError(err, &cacheErr) {
			t.Fatal("Error should be of type CacheError")
		}

		if cacheErr.Operation != OpPut {
			t.Errorf("Expected operation Put, got %v", cacheErr.Operation)
		}

		// Test error unwrapping
		if !errors.Is(err, ErrCacheFull) {
			t.Error("Error should unwrap to ErrCacheFull")
		}

		// Test error context
		context := GetErrorContext(err)
		if context["operation"] != string(OpPut) {
			t.Error("Context should contain operation")
		}
	})

	t.Run("error details extraction", func(t *testing.T) {
		err := WrapPutError(ErrTooLargeData, "/test/path", 2048).
			WithDetails("max_size=1024")

		errStr := err.Error()
		expectedParts := []string{
			"Put",
			"path=/test/path",
			"data_size=2048",
			"max_size=1024",
			"too large data",
		}

		for _, part := range expectedParts {
			if !strings.Contains(errStr, part) {
				t.Errorf("Error string should contain %q, got: %s", part, errStr)
			}
		}
	})
}

// Benchmark for error creation overhead
func BenchmarkCacheError_Creation(b *testing.B) {
	baseErr := errors.New("test error")

	b.Run("NewCacheError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewCacheError(OpPut, baseErr, "test details")
		}
	})

	b.Run("WrapPutError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = WrapPutError(baseErr, "/test/path", 1024)
		}
	})

	b.Run("ErrorString", func(b *testing.B) {
		err := WrapPutError(baseErr, "/test/path", 1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = err.Error()
		}
	})
}

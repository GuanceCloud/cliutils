package parsetoolkit

import (
	"fmt"
	"testing"
)

func TestFormatDuration(t *testing.T) {
	s := FormatDuration(100_000_000_123)
	fmt.Println(s)
}

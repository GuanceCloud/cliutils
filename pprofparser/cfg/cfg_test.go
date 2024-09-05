package cfg

import (
	"fmt"
	"testing"
)

func TestInitConfig(t *testing.T) {
	err := Load("testdata/conf.yml")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%+#v\n", Cfg)
}

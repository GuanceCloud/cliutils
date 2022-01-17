package luascript

import (
	"testing"
	"time"
)

func TestCron(t *testing.T) {
	luaCode := `
	print("---------- lua ---------")
	`

	c := NewLuaCron()
	err := c.AddLua(luaCode, "*/1 * * * *")
	if err != nil {
		t.Fatal(err)
	}

	c.Run()
	defer c.Stoping()

	time.Sleep(time.Second * 5)
}

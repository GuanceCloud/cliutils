package luascript

import (
	"testing"
	"time"

	"github.com/robfig/cron"
)

func TestCronLua(t *testing.T) {
	luaCode := `
	print("---------- lua ---------")
	`

	specParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month)
	sched, err := specParser.Parse("*/1 * * * *")
	if err != nil {
		t.Fatal(err)
	}

	c := NewCronLua()
	c.AppendHandle(CronHandle{
		Code:  luaCode,
		Sched: sched,
	})
	c.Run()
	time.Sleep(time.Second * 5)
}

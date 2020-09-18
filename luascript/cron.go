package luascript

import (
	"log"

	"github.com/robfig/cron"
	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/luamode"
)

type CronHandle struct {
	Code  string
	Sched cron.Schedule
}

type CronLua struct {
	crontab *cron.Cron
	Handles []CronHandle
}

func NewCronLua() CronLua {
	return CronLua{
		crontab: cron.New(),
		Handles: []CronHandle{},
	}
}

func (cs *CronLua) Run() {
	for _, c := range cs.Handles {

		l := luamode.NewLuaMode()
		l.RegisterFuncs()
		l.RegisterCacheFuncs(cache)

		code := c.Code
		cs.crontab.Schedule(c.Sched, cron.FuncJob(func() {
			if err := l.DoString(code); err != nil {
				log.Fatalf("[fatal] should not been here: %s", err.Error())
			}
		}))
	}

	cs.crontab.Start()
}

func (cs *CronLua) AppendHandle(h CronHandle) {
	cs.Handles = append(cs.Handles, h)
}

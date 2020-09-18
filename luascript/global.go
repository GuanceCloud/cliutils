package luascript

import (
	"io/ioutil"
	"log"
	"path"

	"github.com/robfig/cron"
	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/cfg"
	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/luamode"
	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/utils"
)

func globalLua() error {

	if len(cfg.Cfg.GlobalLua) == 0 {
		return nil
	}

	glua := NewCronLua()
	specParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month)

	for _, lc := range cfg.Cfg.GlobalLua {
		code, err := ioutil.ReadFile(path.Join(cfg.DWLuaPath, lc.Path))
		if err != nil {
			log.Printf("[error] global lua read file %s failed: %s", lc.Path, err.Error())
			continue
		}

		if err := luamode.LoadString(string(code)); err != nil {
			log.Printf("[error] global lua load %s failed: %s", lc.Path, err.Error())
			continue
		}

		sched, err := specParser.Parse(lc.Circle)
		if err != nil {
			log.Printf("[error] global parse circle %s failed: %s", lc.Circle, err.Error())
			continue
		}

		glua.AppendHandle(CronHandle{
			Code:  utils.Bytes2String(code),
			Sched: sched,
		})

	}

	glua.Run()

	log.Printf("[info] global lua start worker jobs: %d", len(glua.Handles))

	return nil
}

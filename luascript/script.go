package luascript

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/luascript/module"
)

const defaultWorkerNum = 4

type LuaData interface {
	Handle(value string, err error)
	DataToLua() interface{}
	Name() string
	CallbackFnName() string
	CallbackTypeName() string
}

var defaultLuaScript = &LuaScript{
	codes:     make(map[string][]string),
	workerNum: defaultWorkerNum,
	luaCache:  &module.LuaCache{},
	dataChan:  make(chan LuaData, defaultWorkerNum*2),
	exit:      cliutils.NewSem(),
}

type LuaScript struct {
	codes     map[string][]string
	workerNum int
	luaCache  *module.LuaCache
	dataChan  chan LuaData
	exit      *cliutils.Sem
}

func NewLuaScript(workerNum int) *LuaScript {
	return &LuaScript{
		codes:     make(map[string][]string),
		workerNum: workerNum,
		luaCache:  &module.LuaCache{},
		dataChan:  make(chan LuaData, workerNum*2),
		exit:      cliutils.NewSem(),
	}
}

func (s *LuaScript) AddLuaVMs(name string, codes []string) error {
	if _, ok := s.codes[name]; ok {
		return fmt.Errorf("the %s runner line already exist", name)
	}

	for _, code := range codes {
		if err := CheckLuaCode(code); err != nil {
			return err
		}
	}
	s.codes[name] = codes
	return nil
}

func (s *LuaScript) Run() {
	for i := 0; i < s.workerNum; i++ {
		go func() {
			wk := newWork(s, s.codes)
			wk.run()
		}()
	}
}

func (s *LuaScript) Stop() {
	s.exit.Close()
}

type worker struct {
	script *LuaScript
	ls     map[string][]*lua.LState
}

func newWork(script *LuaScript, lines map[string][]string) *worker {
	wk := &worker{
		script: script,
		ls:     make(map[string][]*lua.LState),
	}
	for name, codes := range lines {
		lst := []*lua.LState{}
		for _, code := range codes {
			luastate := lua.NewState()
			module.RegisterAllFuncs(luastate, wk.script.luaCache, nil)

			luastate.DoString(code)
			lst = append(lst, luastate)
		}
		wk.ls[name] = lst
	}
	return wk
}

func (wk *worker) run() {
	for {
	AGAIN:
		select {
		case data := <-wk.script.dataChan:
			ls := wk.ls[data.Name()]
			if len(ls) == 0 {
				data.Handle("", fmt.Errorf("not found LuaState for this name"))
				goto AGAIN
			}

			var err error
			val := lua.LNil
			for index, l := range ls {
				if index == 0 {
					val = ToLValue(l, data.DataToLua())
				}
				val, err = SendToLua(l, val, data.CallbackFnName(), data.CallbackTypeName())
				if err != nil {
					data.Handle("", fmt.Errorf("luaState exec error: %v", err))
					goto AGAIN
				}
			}
			data.Handle(val.String(), nil)

		case <-wk.script.exit.Wait():
			wk.clean()
			return
		}
	}
}

func (wk *worker) clean() {
	for _, ls := range wk.ls {
		for _, luastate := range ls {
			luastate.Close()
		}
	}
}

func CheckLuaCode(code string) error {
	reader := strings.NewReader(code)
	_, err := parse.Parse(reader, "<string>")
	return err
}

package luascript

import (
	"fmt"
	"strings"
	"sync"

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
	runStatus: false,
	wg:        sync.WaitGroup{},
}

func AddLuaLines(name string, codes []string) error {
	return defaultLuaScript.AddLuaLines(name, codes)
}

func Run() {
	defaultLuaScript.Run()
}

func SendData(d LuaData) error {
	return defaultLuaScript.SendData(d)
}

func Stop() {
	defaultLuaScript.Stop()
}

type LuaScript struct {
	codes     map[string][]string
	workerNum int
	dataChan  chan LuaData

	luaCache *module.LuaCache

	exit      *cliutils.Sem
	runStatus bool
	wg        sync.WaitGroup
}

func NewLuaScript(workerNum int) *LuaScript {
	return &LuaScript{
		codes:     make(map[string][]string),
		workerNum: workerNum,
		dataChan:  make(chan LuaData, workerNum*2),
		luaCache:  &module.LuaCache{},
		runStatus: false,
		wg:        sync.WaitGroup{},
	}
}

func (s *LuaScript) AddLuaLines(name string, codes []string) error {
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
	if s.runStatus {
		return
	}

	s.exit = cliutils.NewSem()

	for i := 0; i < s.workerNum; i++ {
		s.wg.Add(1)
		go func() {
			wk := newWork(s, s.codes)
			wk.run()
			s.wg.Done()
		}()
	}

	s.runStatus = true
}

func (s *LuaScript) SendData(d LuaData) error {
	// channel already close?
	if _, ok := s.codes[d.Name()]; !ok {
		return fmt.Errorf("not found luaState of this name '%s'", d.Name())
	}
	s.dataChan <- d
	return nil
}

func (s *LuaScript) Stop() {
	if !s.runStatus {
		return
	}
	s.exit.Close()
	s.wg.Wait()
	s.runStatus = false
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
			var err error
			ls := wk.ls[data.Name()]
			val := lua.LNil

			for index, l := range ls {
				if index == 0 {
					val = ToLValue(l, data.DataToLua())
				}
				val, err = SendToLua(l, val, data.CallbackFnName(), data.CallbackTypeName())
				if err != nil {
					data.Handle("", fmt.Errorf("lua '%s' exec error: %v", data.Name(), err))
					goto AGAIN
				}
			}

			jsonStr, err := JsonEncode(val)
			if err != nil {
				data.Handle("", fmt.Errorf("lua '%s' exec error: %v", data.Name(), err))
				goto AGAIN
			}

			data.Handle(jsonStr, nil)

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

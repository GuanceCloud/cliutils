package luascript

import (
	"fmt"
	"sync"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"
	lua "github.com/yuin/gopher-lua"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/luascript/module"
)

const (
	defaultWorkNum = 4

	defaultEnableStrongType = true

	defaultWorkingTimeout = time.Second * 6
)

type pointsReq struct {
	name string
	pts  []*influxdb.Point
	ch   chan interface{}
}

var (
	pointsChan = make(chan *pointsReq, defaultWorkNum*2)
)

type work struct {
	ls      map[string][]*lua.LState
	typelog map[string]fieldType
	opt     *Option
}

func newWork(lines map[string][]string, opt *Option) (*work, error) {
	wk := &work{
		ls:      make(map[string][]*lua.LState),
		typelog: make(map[string]fieldType),
		opt:     opt,
	}

	for name, codes := range lines {
		lst := []*lua.LState{}

		for _, code := range codes {
			luastate := lua.NewState()
			module.RegisterAllFuncs(luastate, luaCache, nil)

			if err := luastate.DoString(code); err != nil {
				return nil, err
			}

			lst = append(lst, luastate)
		}

		wk.ls[name] = lst
	}

	return wk, nil
}

func (wk *work) run() {
	var err error

	for {
	AGAIN:
		select {
		case pd := <-pointsChan:
			ls, ok := wk.ls[pd.name]
			if !ok {
				goto AGAIN
			}
			pts := pd.pts

			if wk.opt.EnableStrongType {
				wk.typelog = logType(pts)
			}

			for _, luastate := range ls {
				pts, err = PointsOnHandle(luastate, pts, wk.typelog)
				if err != nil {
					goto AGAIN
				}
			}

			if wk.opt.EnableStrongType {
				pts, err = typeRecove(pts, wk.typelog)
				if err != nil {
					goto AGAIN
				}
			}

			pd.ch <- pts

		case <-wk.opt.exit.Wait():
			wk.clean()
			return
		}
	}
}

func (wk *work) clean() {
	for _, ls := range wk.ls {
		for _, luastate := range ls {
			luastate.Close()
		}
	}
}

type Option struct {
	WorkNum          int
	EnableStrongType bool
	exit             *cliutils.Sem
}

type LuaScript struct {
	lines map[string][]string
	opt   *Option
	wg    sync.WaitGroup
}

func NewLuaScript(opt ...*Option) *LuaScript {
	s := &LuaScript{
		lines: make(map[string][]string),
		wg:    sync.WaitGroup{},
	}
	if len(opt) > 0 {
		s.opt = opt[0]
	} else {
		s.opt = &Option{
			WorkNum:          defaultWorkNum,
			EnableStrongType: defaultEnableStrongType,
		}
	}
	s.opt.exit = cliutils.NewSem()
	return s
}

func (s *LuaScript) AddLuaCode(name string, codes []string) error {
	if _, ok := s.lines[name]; ok {
		return fmt.Errorf("the %s runner line already exist", name)
	}

	for _, code := range codes {
		if err := CheckLuaCode(code); err != nil {
			return err
		}
	}

	s.lines[name] = codes
	return nil
}

func (s *LuaScript) Run() {
	for i := 0; i < s.opt.WorkNum; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			wk, err := newWork(s.lines, s.opt)
			if err != nil {
				return
			}
			wk.run()
		}()
	}
}

func (s *LuaScript) SendPoints(name string, pts []*influxdb.Point) ([]*influxdb.Point, error) {
	req := &pointsReq{
		name: name,
		pts:  pts,
		ch:   make(chan interface{}),
	}
	pointsChan <- req
	defer close(req.ch)

	select {
	case <-time.After(defaultWorkingTimeout):
		return nil, fmt.Errorf("%s working timeout", name)

	case result := <-req.ch:
		switch t := result.(type) {
		case error:
			return nil, result.(error)
		case []*influxdb.Point:
			return result.([]*influxdb.Point), nil
		default:
			return nil, fmt.Errorf("invalid result type: %v", t)
		}
	}
}

func (s *LuaScript) Stop() {
	s.opt.exit.Close()
}

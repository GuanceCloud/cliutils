package luascript

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/luamode"
	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/utils"

	cutils "gitlab.jiagouyun.com/cloudcare-tools/cliutils"
	"gitlab.jiagouyun.com/cloudcare-tools/ftagent/cfg"
)

type req struct {
	ch chan interface{}

	points []*influxdb.Point
	route  string
}

var (
	sem        *cutils.Sem
	pointsChan chan *req
	wg         = sync.WaitGroup{}

	// use only cache
	cache = luamode.NewCache()
	// use only log io.Writer
	logger = luamode.NewLog(log.Writer())
)

func CheckRouteLua() int {

	// precheck router's lua files
	n := 0
	for _, route := range cfg.Cfg.Routes {
		if len(route.Lua) == 0 {
			continue
		}

		for _, lf := range route.Lua {
			code, err := ioutil.ReadFile(path.Join(cfg.DWLuaPath, lf.Path))
			if err != nil {
				log.Printf("[error] read %s failed under router %s: %s, route's lua disabled",
					lf.Path, route.Name, err.Error())

				route.DisableLua = true
				continue

			}
			if err := luamode.LoadString(string(code)); err != nil {
				log.Printf("[error] load %s failed under router %s: %s, route's lua disabled",
					lf.Path, route.Name, err.Error())

				route.DisableLua = true

				continue
			} else {
				n++
				log.Printf("[info] %s seems ok", lf.Path)
			}
		}
	}

	return n
}

func Start() error {

	sem = cutils.NewSem()

	globalLua()

	nlua := CheckRouteLua()
	if nlua == 0 { // no lua, no worker
		return nil
	}

	nworker := cfg.Cfg.LuaWorker
	if nworker == 0 {
		nworker = 1 // at lease 1 worker
	}

	pointsChan = make(chan *req, nworker*2)

	wg.Add(nworker)
	for i := 0; i < nworker; i++ {
		wkr := &worker{
			idx:       i,
			ls:        map[string][]luamode.LMode{},
			luaFiles:  map[string][]string{},
			typeCheck: map[string]bool{}}
		go wkr.start()
	}

	log.Printf("[info] route lua module start..")
	return nil
}

func Stop() {
	sem.Close()

	log.Printf("[debug] waiting lua workers exit...")
	wg.Wait()
	log.Printf("[debug] all lua workers exit")
}

func doSend(pts []*influxdb.Point, route string) ([]*influxdb.Point, error) {
	r := &req{
		points: pts,
		route:  route,
		ch:     make(chan interface{}),
	}

	log.Printf("[debug] send to lua worker...")
	pointsChan <- r

	defer close(r.ch)

	log.Printf("[debug] wait points from lua worker...")
	select {
	case res := <-r.ch:
		switch res.(type) {
		case error:
			return nil, res.(error)
		case []*influxdb.Point:
			return res.([]*influxdb.Point), nil
		}
	}

	return nil, errors.New("should not been here")
}

func Send(c *gin.Context, pts []*influxdb.Point, route string) ([]*influxdb.Point, error) {

	for _, rt := range cfg.Cfg.Routes {
		if route == rt.Name && len(rt.Lua) > 0 && !rt.DisableLua {
			goto __goon
		}
	}

	log.Printf("[debug] no lua enabled under %s, skipped", route)
	return pts, nil

__goon:
	if pointsChan == nil { // FIXME: is it ok?
		log.Printf("[debug] no lua enabled and skipped")
		return pts, nil
	}

	start := time.Now()
	res, err := doSend(pts, route)

	c.Header("X-Lua", fmt.Sprintf("%s, cost %v, input %d, output %d",
		route, time.Since(start), len(pts), len(res)))
	return res, err
}

type worker struct {
	idx       int
	ls        map[string][]luamode.LMode
	luaFiles  map[string][]string
	typeCheck map[string]bool

	jobs   int64
	failed int64

	// TODO: add each lstate runing-info
}

func (w *worker) start() {

	defer wg.Done()

	w.loadLuas()

	var typelog map[string]fieldType = nil
	var err error

	for {
	__goOn:

		select {
		case pd := <-pointsChan:
			w.jobs++

			if w.jobs%8 == 0 {
				log.Printf("[debug][%d] lua worker jobs: %d, failed: %d", w.idx, w.jobs, w.failed)
			}

			pts := pd.points

			ls, ok := w.ls[pd.route]
			if !ok {
				w.failed++
				log.Printf("[error] router %s not exists", pd.route)

				pd.ch <- utils.ErrLuaRouteNotFound
				break __goOn
			}

			if w.typeCheck[pd.route] {
				typelog = logType(pts) // log type info
			}

			// Send @pts to every lua sequentially
			// XXX: the successive lua handler will overwrite previous @pts
			for idx, l := range ls {

				log.Printf("[debug] send %d pts to %s...",
					len(pts), w.luaFiles[pd.route][idx])

				pts, err = l.PointsOnHandle(pts)
				if err != nil {
					log.Printf("[error] route %s handle PTS failed within %s: %s",
						pd.route, w.luaFiles[pd.route][idx], err.Error())

					w.failed++
					pd.ch <- err
					break __goOn
				}
			}

			if w.typeCheck[pd.route] { // recover type info
				log.Printf("[debug] recover type info under %s", pd.route)
				pts, err = typeRecove(pts, typelog)
				if err != nil {
					w.failed++
					pd.ch <- err
					break __goOn
				}
			}

			pd.ch <- pts

		case <-sem.Wait():
			log.Printf("[info][%d] lua worker exit on sem", w.idx)
			return
		}
	}
}

// log only `int' fields
type fieldType []string

func logType(pts []*influxdb.Point) map[string]fieldType {
	fts := map[string]fieldType{}

	for _, p := range pts {
		fts[p.Name()] = filterIntFields(p)
	}

	return fts
}

func (w *worker) loadLuas() {

	for _, r := range cfg.Cfg.Routes {
		if len(r.Lua) == 0 || r.DisableLua {
			continue
		}

		w.typeCheck[r.Name] = !r.DisableTypeCheck

		if _, ok := w.ls[r.Name]; !ok { // create route entry
			w.ls[r.Name] = []luamode.LMode{}
			w.luaFiles[r.Name] = []string{}
		}

		// NOTE: router's lua list is order-sensitive, they
		// seems like a stream-line to handle the input PTS
		for _, rl := range r.Lua {
			l := luamode.NewLuaMode()
			if err := l.DoFile(path.Join(cfg.DWLuaPath, rl.Path)); err != nil {
				log.Fatalf("[fatal] should not been here: %s", err.Error())
			}

			l.RegisterFuncs()
			l.RegisterCacheFuncs(cache)
			l.RegisterLogFuncs(logger)

			w.ls[r.Name] = append(w.ls[r.Name], l) // add new lua-state to route
			w.luaFiles[r.Name] = append(w.luaFiles[r.Name], rl.Path)
		}
	}
}

func typeRecove(pts []*influxdb.Point, typelog map[string]fieldType) ([]*influxdb.Point, error) {
	var points []*influxdb.Point

	for _, pt := range pts {
		newpt, err := recoverIntFields(pt, typelog[pt.Name()])
		if err != nil {
			return nil, err
		}
		points = append(points, newpt)
	}
	return points, nil
}

func recoverIntFields(p *influxdb.Point, ft fieldType) (*influxdb.Point, error) {

	if len(ft) == 0 { // FIXME: need new point based on @p?
		return p, nil
	}

	fs, err := p.Fields()
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return nil, utils.ErrLuaInvalidPoints
	}

	pn := p.Name()

	n := 0

	// NOTE: Lua do not distinguish int/float, all Golang got is float.
	// if your really need int to be float, disable type-safe in configure.
	// Loop all original int fields, they must be float now, convert to int anyway.
	// We do not check other types of fields, the Lua developer SHOULD becarefull
	// to treat them type-safe when updating exists field values, or influxdb
	// may refuse to accept the point handled by Lua.
	for _, k := range ft {

		if fs[k] == nil {
			log.Printf("[debug] ignore missing filed %s.%s", pn, k)
			continue
		}

		switch fs[k].(type) {
		case float32:
			fs[k] = int64(fs[k].(float32))
			n++
		case float64:
			fs[k] = int64(fs[k].(float64))
			n++
		default:
			log.Printf("[warn] overwrite int field(%s.%s) with conflict type: int > %v, point: %s, ft: %v",
				pn, k, fs[k], p.String(), ft)
		}
	}

	if n == 0 { // no field updated
		return p, nil
	} else {

		log.Printf("[debug] %d points type recovered", n)

		pt, err := influxdb.NewPoint(pn, p.Tags(), fs, p.Time())
		if err != nil {
			log.Printf("[error] %s", err.Error())
			return nil, err
		}

		return pt, nil
	}
}

func filterIntFields(pt *influxdb.Point) fieldType {
	ft := fieldType{}
	fs, err := pt.Fields()
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return nil
	}

	for k, v := range fs {
		switch v.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			ft = append(ft, k)
		}
	}

	return ft
}

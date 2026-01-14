package aggregate

import (
	"github.com/GuanceCloud/cliutils/point"
	"sync"
	"time"
)

type Window struct {
	lock  sync.Mutex // 为每一个window创建一把锁
	cache map[uint64]Calculator
	Token string // 用户唯一标记
}

func (w *Window) AddCal(cal Calculator) {
	w.lock.Lock()
	defer w.lock.Unlock()
	calcHash := cal.Base().hash
	if calc, ok := w.cache[calcHash]; ok {
		calc.Add(cal)
		//		l.Debugf("append to instance %s, heap size %d", cal.Base())
	} else {
		cal.Base().build()
		w.cache[calcHash] = cal
		//		l.Debugf("create new instance %s, heap size %d", cal.Base())
	}
}

type Windows struct {
	lock sync.Mutex
	// 为方便快速定位到用户数据的所在的window需要一个ID表
	// token -> Window ID
	IDs map[string]int
	// WindowID -> Window
	WS []*Window
}

func (ws *Windows) AddCal(token string, cal Calculator) {
	ws.lock.Lock()
	id, ok := ws.IDs[token]
	if !ok {
		ws.IDs[token] = len(ws.WS)
		ws.WS = append(ws.WS, &Window{
			cache: make(map[uint64]Calculator),
			Token: token,
		})
	}
	ws.lock.Unlock()

	ws.WS[id].AddCal(cal)
}

type Cache struct {
	lock sync.Mutex
	// 每一个窗口创建一个对象，针对这个Window 进行add 操作，最终到达容忍时间，整个windows会从map中删除
	// key:容忍时间+窗口时间。
	WindowsBuckets map[int64]*Windows
	//容忍时间
	Expired time.Duration
}

func NewCache(exp time.Duration) *Cache {
	return &Cache{
		WindowsBuckets: make(map[int64]*Windows),
		Expired:        exp,
	}
}

func (c *Cache) GetAndSetBucket(exp int64, token string, cal Calculator) {
	c.lock.Lock()
	ws, ok := c.WindowsBuckets[exp]
	if !ok {
		ws = &Windows{IDs: make(map[string]int), WS: make([]*Window, 0)}
		c.WindowsBuckets[exp] = ws
	}
	c.lock.Unlock()
	ws.AddCal(token, cal)
}

func (c *Cache) AddBatch(token string, batch *AggregationBatch) (n, expN int) {
	nowTime := time.Now().Unix()
	for _, cal := range newCalculators(batch) {
		exp := cal.Base().nextWallTime + int64(c.Expired/time.Second)
		if nowTime >= exp {
			expN++
			continue
		}
		c.GetAndSetBucket(exp, token, cal)
		n++
	}
	return n, expN
}

func (c *Cache) GetExpWidows() []*Window {
	var wss []*Window
	c.lock.Lock()
	defer c.lock.Unlock()
	now := time.Now().Unix()
	for t, ws := range c.WindowsBuckets {
		if t <= now {
			for _, w := range ws.WS {
				wss = append(wss, w)
			}
			delete(c.WindowsBuckets, t)
		}
	}

	return wss
}

type PointsData struct {
	PTS   []*point.Point
	Token string
}

func WindowsToData(ws []*Window) []*PointsData {
	pds := make([]*PointsData, 0)
	for _, window := range ws {
		var pts []*point.Point
		for _, cal := range window.cache {
			pbs, err := cal.Aggr()
			if err != nil {
				l.Warnf("aggr err =%w", err)
				continue
			}
			pts = append(pts, pbs...)
		}
		// 每一个用户下的Window 都是一个独立的包
		pds = append(pds, &PointsData{
			PTS:   pts,
			Token: window.Token,
		})
	}

	return pds
}

// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils/point"
)

const (
	windowCacheReuseMaxEntries     = 4096
	aggregationBucketBaseBytes     = 256
	aggregationWindowBaseBytes     = 512
	aggregationCalculatorBaseBytes = 384
)

// CacheOptions controls the optional state admission of an aggregation cache.
// Zero MaxWindowSpan retains the historical unbounded window-span behavior.
type CacheOptions struct {
	StateBudget   StateBudget
	MaxWindowSpan time.Duration
}

// AggregationAdmissionResult describes the fate of calculators in one batch.
// Dropped counts whole windows, while PrecisionDegraded means the window stays
// complete but a quantile/count-distinct calculator stopped retaining detail.
type AggregationAdmissionResult struct {
	Accepted           int
	Expired            int
	Dropped            int
	PrecisionDegraded  int
	WindowSpanExceeded int
	Rejection          *StateBudgetError
}

type windowAdmissionResult struct {
	accepted          bool
	dropped           bool
	newlyDropped      bool
	precisionDegraded bool
	rejection         *StateBudgetError
	closed            bool
}

type Window struct {
	lock  sync.Mutex // 为每一个window创建一把锁
	cache map[uint64]Calculator
	Token string // 用户唯一标记

	budget      StateBudget
	windowLease *StateLease
	pointLease  *StateLease
	calcLeases  map[uint64]*StateLease
	calcCosts   map[uint64]StateCost
	dropped     bool
	rejection   *StateBudgetError
}

// Reset prepares a window for reuse. It also releases any state that remains
// attached to a caller that did not pass the window through WindowsToData.
func (w *Window) Reset() {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.releaseStateLocked()
	w.resetLocked()
}

func (w *Window) resetLocked() {
	w.Token = ""
	w.budget = nil
	w.windowLease = nil
	w.pointLease = nil
	w.calcLeases = nil
	w.calcCosts = nil
	w.dropped = false
	w.rejection = nil
	if w.cache == nil {
		w.cache = make(map[uint64]Calculator, 64)
		return
	}
	if len(w.cache) > windowCacheReuseMaxEntries {
		w.cache = make(map[uint64]Calculator, 64)
		return
	}
	for key := range w.cache {
		delete(w.cache, key)
	}
}

// AddCal keeps the legacy API and discards an optional state-budget result.
func (w *Window) AddCal(cal Calculator) {
	_ = w.addCal(cal)
}

func (w *Window) addCal(cal Calculator) windowAdmissionResult {
	if cal == nil {
		return windowAdmissionResult{}
	}
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.dropped {
		return windowAdmissionResult{dropped: true, rejection: w.rejection}
	}

	if rejection := w.reservePointLocked(); rejection != nil {
		w.dropLocked(rejection)
		return windowAdmissionResult{dropped: true, newlyDropped: true, rejection: rejection}
	}

	calcHash := cal.Base().hash
	if existing, ok := w.cache[calcHash]; ok {
		return w.mergeCalculatorLocked(existing, cal, calcHash)
	}

	cost := estimateCalculatorCost(cal)
	lease, rejection := w.reserveCalculatorLocked(cost)
	if rejection != nil {
		w.dropLocked(rejection)
		return windowAdmissionResult{dropped: true, newlyDropped: true, rejection: rejection}
	}
	cal.Base().build()
	w.cache[calcHash] = cal
	if lease != nil {
		if w.calcLeases == nil {
			w.calcLeases = make(map[uint64]*StateLease)
			w.calcCosts = make(map[uint64]StateCost)
		}
		w.calcLeases[calcHash] = lease
		w.calcCosts[calcHash] = cost
	}
	return windowAdmissionResult{accepted: true}
}

func (w *Window) mergeCalculatorLocked(existing, incoming Calculator, calcHash uint64) windowAdmissionResult {
	current := w.calcCosts[calcHash]
	if current == (StateCost{}) {
		current = estimateCalculatorCost(existing)
	}
	next, precisionSensitive := calculatorNextCost(existing, incoming, current)
	if lease := w.calcLeases[calcHash]; lease != nil && next != current {
		if rejection := w.budget.Resize(lease, next); rejection != nil {
			if precisionSensitive {
				mergeCalculatorWithDegradedPrecision(existing, incoming)
				return windowAdmissionResult{accepted: true, precisionDegraded: true, rejection: rejection}
			}
			w.dropLocked(rejection)
			return windowAdmissionResult{dropped: true, newlyDropped: true, rejection: rejection}
		}
		w.calcCosts[calcHash] = next
	}
	existing.Add(incoming)
	return windowAdmissionResult{accepted: true}
}

func (w *Window) reservePointLocked() *StateBudgetError {
	if w.budget == nil {
		return nil
	}
	if w.pointLease == nil {
		lease, rejection := w.budget.Reserve(StateReservation{
			Workspace: w.Token,
			Kind:      StateKindAggregationPoint,
			Cost:      StateCost{Objects: 1},
		})
		if rejection != nil {
			return rejection
		}
		w.pointLease = lease
		return nil
	}
	cost := w.pointLease.Reservation().Cost
	cost.Objects++
	return w.budget.Resize(w.pointLease, cost)
}

func (w *Window) reserveCalculatorLocked(cost StateCost) (*StateLease, *StateBudgetError) {
	if w.budget == nil {
		return nil, nil
	}
	return w.budget.Reserve(StateReservation{
		Workspace: w.Token,
		Kind:      StateKindAggregationCalculator,
		Cost:      cost,
	})
}

func (w *Window) dropLocked(rejection *StateBudgetError) {
	w.dropped = true
	w.rejection = rejection
	if w.budget != nil {
		w.budget.Release(w.pointLease)
		for _, lease := range w.calcLeases {
			w.budget.Release(lease)
		}
	}
	w.pointLease = nil
	w.calcLeases = nil
	w.calcCosts = nil
	for key := range w.cache {
		delete(w.cache, key)
	}
}

func (w *Window) releaseStateLocked() {
	if w.budget != nil {
		w.budget.Release(w.windowLease)
		w.budget.Release(w.pointLease)
		for _, lease := range w.calcLeases {
			w.budget.Release(lease)
		}
	}
}

type Windows struct {
	lock   sync.Mutex
	closed bool
	// 为方便快速定位到用户数据的所在的window需要一个ID表
	// token -> Window ID
	IDs map[string]int
	// WindowID -> Window
	WS []*Window

	budget      StateBudget
	bucketLease *StateLease
}

// AddCal keeps the legacy API and discards the optional admission result.
func (ws *Windows) AddCal(token string, cal Calculator) {
	_ = ws.addCal(token, cal)
}

func (ws *Windows) addCal(token string, cal Calculator) windowAdmissionResult {
	ws.lock.Lock()
	defer ws.lock.Unlock()
	if ws.closed {
		return windowAdmissionResult{closed: true}
	}

	id, ok := ws.IDs[token]
	created := false
	if !ok {
		created = true
		windowLease, rejection := ws.reserveWindowLocked(token)
		newWindow := windowPool.Get().(*Window) //nolint:forcetypeassert
		newWindow.Reset()
		newWindow.Token = token
		newWindow.budget = ws.budget
		newWindow.windowLease = windowLease
		if rejection != nil {
			newWindow.dropped = true
			newWindow.rejection = rejection
		}

		id = len(ws.WS)
		ws.IDs[token] = id
		ws.WS = append(ws.WS, newWindow)
	}

	window := ws.WS[id]
	if window.dropped {
		return windowAdmissionResult{dropped: true, newlyDropped: created, rejection: window.rejection}
	}
	return window.addCal(cal)
}

func (ws *Windows) reserveWindowLocked(token string) (*StateLease, *StateBudgetError) {
	if ws.budget == nil {
		return nil, nil
	}
	return ws.budget.Reserve(StateReservation{
		Workspace: token,
		Kind:      StateKindAggregationWindow,
		Cost:      StateCost{Bytes: aggregationWindowBaseBytes, Objects: 1},
	})
}

func (ws *Windows) Close() []*Window {
	ws.lock.Lock()
	defer ws.lock.Unlock()

	ws.closed = true
	windows := append([]*Window(nil), ws.WS...)
	ws.IDs = nil
	ws.WS = nil
	if ws.budget != nil {
		ws.budget.Release(ws.bucketLease)
	}
	ws.bucketLease = nil
	return windows
}

type Cache struct {
	lock sync.Mutex
	// 每一个窗口创建一个对象，针对这个Window 进行add 操作，最终到达容忍时间，整个windows会从map中删除
	// key:容忍时间+窗口时间。
	WindowsBuckets map[int64]*Windows

	Expired       time.Duration
	budget        StateBudget
	maxWindowSpan time.Duration
}

func NewCache(exp time.Duration) *Cache {
	return NewCacheWithOptions(exp, CacheOptions{})
}

// NewCacheWithOptions creates an aggregation cache with optional retained-state admission.
func NewCacheWithOptions(exp time.Duration, options CacheOptions) *Cache {
	return &Cache{
		WindowsBuckets: make(map[int64]*Windows),
		Expired:        exp,
		budget:         options.StateBudget,
		maxWindowSpan:  options.MaxWindowSpan,
	}
}

// StateBudgetSnapshot returns the state-budget view used by this cache.
func (c *Cache) StateBudgetSnapshot() StateBudgetSnapshot {
	if c == nil || c.budget == nil {
		return StateBudgetSnapshot{ByKind: map[StateKind]StateUsage{}}
	}
	return c.budget.Snapshot()
}

func (c *Cache) GetAndSetBucket(exp int64, token string, cal Calculator) {
	_, _ = c.getAndSetBucket(exp, token, cal)
}

func (c *Cache) getAndSetBucket(exp int64, token string, cal Calculator) (windowAdmissionResult, bool) {
	for {
		c.lock.Lock()
		ws, ok := c.WindowsBuckets[exp]
		if !ok {
			bucketLease, rejection := c.reserveBucketLocked()
			if rejection != nil {
				c.lock.Unlock()
				return windowAdmissionResult{dropped: true, newlyDropped: true, rejection: rejection}, false
			}
			ws = &Windows{
				IDs:         make(map[string]int),
				WS:          make([]*Window, 0),
				budget:      c.budget,
				bucketLease: bucketLease,
			}
			c.WindowsBuckets[exp] = ws
		}
		c.lock.Unlock()

		result := ws.addCal(token, cal)
		if !result.closed {
			return result, false
		}
		if time.Now().Unix() >= exp {
			return windowAdmissionResult{}, true
		}
	}
}

func (c *Cache) reserveBucketLocked() (*StateLease, *StateBudgetError) {
	if c.budget == nil {
		return nil, nil
	}
	return c.budget.Reserve(StateReservation{
		Kind: StateKindAggregationBucket,
		Cost: StateCost{Bytes: aggregationBucketBaseBytes, Objects: 1},
	})
}

func (c *Cache) AddBatch(token string, batch *AggregationBatch) (n, expN int) {
	result := c.AddBatchWithResult(token, batch)
	return result.Accepted, result.Expired
}

// AddBatchWithResult adds a batch and returns state-admission information for callers
// that need to expose backpressure. A dropped window remains a tombstone until expiry,
// so it cannot later produce a partial aggregation result.
func (c *Cache) AddBatchWithResult(token string, batch *AggregationBatch) AggregationAdmissionResult {
	var result AggregationAdmissionResult
	nowTime := time.Now().Unix()
	for _, cal := range newCalculators(batch) {
		if c.windowSpanExceeded(cal) {
			result.WindowSpanExceeded++
			continue
		}
		exp := cal.Base().nextWallTime + int64(c.Expired/time.Second)
		if nowTime >= exp {
			result.Expired++
			continue
		}
		admission, expired := c.getAndSetBucket(exp, token, cal)
		if expired {
			result.Expired++
			continue
		}
		if admission.accepted {
			result.Accepted++
		}
		if admission.newlyDropped {
			result.Dropped++
		}
		if admission.precisionDegraded {
			result.PrecisionDegraded++
		}
		if result.Rejection == nil && admission.rejection != nil {
			result.Rejection = admission.rejection
		}
	}
	return result
}

func (c *Cache) windowSpanExceeded(cal Calculator) bool {
	return c.maxWindowSpan > 0 && time.Duration(cal.Base().window) > c.maxWindowSpan
}

func (c *Cache) AddBatchs(token string, batchs []*AggregationBatch) (n, expN int) {
	result := c.AddBatchsWithResult(token, batchs)
	return result.Accepted, result.Expired
}

// AddBatchsWithResult is the multi-batch counterpart of AddBatchWithResult.
func (c *Cache) AddBatchsWithResult(token string, batchs []*AggregationBatch) AggregationAdmissionResult {
	var result AggregationAdmissionResult
	for _, batch := range batchs {
		one := c.AddBatchWithResult(token, batch)
		result.Accepted += one.Accepted
		result.Expired += one.Expired
		result.Dropped += one.Dropped
		result.PrecisionDegraded += one.PrecisionDegraded
		result.WindowSpanExceeded += one.WindowSpanExceeded
		if result.Rejection == nil && one.Rejection != nil {
			result.Rejection = one.Rejection
		}
	}
	return result
}

func (c *Cache) GetExpWidows() []*Window {
	var windows []*Window
	c.lock.Lock()
	defer c.lock.Unlock()
	now := time.Now().Unix()
	for expiration, ws := range c.WindowsBuckets {
		if expiration <= now {
			windows = append(windows, ws.Close()...)
			delete(c.WindowsBuckets, expiration)
		}
	}
	return windows
}

// Close releases all aggregation state without producing output.
func (c *Cache) Close() {
	if c == nil {
		return
	}
	c.lock.Lock()
	buckets := c.WindowsBuckets
	c.WindowsBuckets = make(map[int64]*Windows)
	c.lock.Unlock()
	for _, bucket := range buckets {
		for _, window := range bucket.Close() {
			window.Reset()
			windowPool.Put(window)
		}
	}
}

type PointsData struct {
	PTS   []*point.Point
	Token string
}

func WindowsToData(windows []*Window) []*PointsData {
	pointsData := make([]*PointsData, 0)
	for _, window := range windows {
		if window == nil {
			continue
		}
		if window.dropped {
			window.Reset()
			windowPool.Put(window)
			continue
		}

		var points []*point.Point
		for _, cal := range window.cache {
			pbs, err := cal.Aggr()
			if err != nil {
				l.Warnf("aggr err =%w", err)
				continue
			}
			points = append(points, pbs...)
		}
		if len(points) > 0 {
			pointsData = append(pointsData, &PointsData{PTS: points, Token: window.Token})
		}
		window.Reset()
		windowPool.Put(window)
	}
	return pointsData
}

func estimateCalculatorCost(cal Calculator) StateCost {
	if cal == nil {
		return StateCost{}
	}
	base := cal.Base()
	bytes := int64(aggregationCalculatorBaseBytes + len(base.key) + len(base.name))
	for _, tag := range base.aggrTags {
		bytes += int64(len(tag[0]) + len(tag[1]))
	}
	if base.pt != nil {
		bytes += int64(base.pt.Size())
	}
	switch typed := cal.(type) {
	case *algoQuantiles:
		bytes += int64(len(typed.all)+len(typed.quantiles)) * 8
	case *algoCountDistinct:
		if typed.sketch != nil {
			bytes += int64(len(typed.sketch)) * 8
		} else {
			bytes += int64(len(typed.distinctValues)) * 24
		}
	case *algoHistogram:
		for label := range typed.leBucket {
			bytes += int64(32 + len(label))
		}
	}
	return StateCost{Bytes: bytes, Objects: 1}
}

func calculatorNextCost(existing, incoming Calculator, current StateCost) (StateCost, bool) {
	switch currentCalc := existing.(type) {
	case *algoQuantiles:
		incomingCalc, ok := incoming.(*algoQuantiles)
		if !ok || len(currentCalc.all) >= quantileSampleLimit {
			return current, true
		}
		growth := minInt(len(incomingCalc.all), quantileSampleLimit-len(currentCalc.all))
		return StateCost{Bytes: current.Bytes + int64(growth)*8, Objects: current.Objects}, true
	case *algoCountDistinct:
		incomingCalc, ok := incoming.(*algoCountDistinct)
		if !ok || currentCalc.sketch != nil {
			return current, true
		}
		if incomingCalc.sketch != nil {
			return StateCost{
				Bytes:   current.Bytes + int64(countDistinctSketchBits/8) - int64(len(currentCalc.distinctValues))*24,
				Objects: current.Objects,
			}, true
		}
		newValues := 0
		for value := range incomingCalc.distinctValues {
			if _, exists := currentCalc.distinctValues[value]; !exists {
				newValues++
			}
		}
		if newValues == 0 {
			return current, true
		}
		if len(currentCalc.distinctValues)+newValues <= countDistinctExactLimit {
			return StateCost{Bytes: current.Bytes + int64(newValues)*24, Objects: current.Objects}, true
		}
		return StateCost{
			Bytes:   current.Bytes + int64(countDistinctSketchBits/8) - int64(len(currentCalc.distinctValues))*24,
			Objects: current.Objects,
		}, true
	case *algoHistogram:
		incomingCalc, ok := incoming.(*algoHistogram)
		if !ok {
			return current, false
		}
		growth := int64(0)
		for label := range incomingCalc.leBucket {
			if _, exists := currentCalc.leBucket[label]; !exists {
				growth += int64(32 + len(label))
			}
		}
		return StateCost{Bytes: current.Bytes + growth, Objects: current.Objects}, false
	default:
		return current, false
	}
}

func mergeCalculatorWithDegradedPrecision(existing, incoming Calculator) {
	switch currentCalc := existing.(type) {
	case *algoQuantiles:
		if incomingCalc, ok := incoming.(*algoQuantiles); ok {
			currentCalc.count += incomingCalc.count
			if incomingCalc.maxTime > currentCalc.maxTime {
				currentCalc.maxTime = incomingCalc.maxTime
			}
		}
	case *algoCountDistinct:
		if incomingCalc, ok := incoming.(*algoCountDistinct); ok && incomingCalc.maxTime > currentCalc.maxTime {
			currentCalc.maxTime = incomingCalc.maxTime
		}
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

var windowPool = sync.Pool{
	New: func() any {
		return &Window{cache: make(map[uint64]Calculator, 64)}
	},
}

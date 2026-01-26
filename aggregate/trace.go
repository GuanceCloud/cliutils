package aggregate

import (
	"container/list"
	"hash/fnv"
	"sync"
	"time"
)

var tracePool = sync.Pool{
	New: func() interface{} {
		return &TraceData{}
	},
}

// 1. 定义 TraceData 结构
type TraceData struct {
	td          *TraceDataPacket
	FirstSeen   time.Time
	ExpiredTime int64
	slotIndex   int
	element     *list.Element
}

// Reset 清理函数
func (td *TraceData) Reset() {
	td.td = nil // 断开对网络包的引用，方便 GC
	td.element = nil
	td.slotIndex = 0
	td.ExpiredTime = 0
}

// 2. 定义分段桶 (Shard)
type Shard struct {
	mu        sync.Mutex
	activeMap map[uint64]*TraceData

	// 时间轮：本质是一个环形数组
	// 假设最大支持 3600 秒（1小时）的过期时间
	slots      [3600]*list.List
	currentPos int // 当前指针指向的槽位下标
}

// 3. 全局管理器
type GlobalSampler struct {
	shards      []*Shard
	shardCount  int
	waitTime    time.Duration // 5分钟
	configMap   map[string]*TailSampling
	lock        sync.RWMutex
	exportTrace []*TraceData
}

func NewGlobalSampler(shardCount int, waitTime time.Duration) *GlobalSampler {
	return &GlobalSampler{
		shards:     make([]*Shard, shardCount),
		shardCount: shardCount,
		waitTime:   waitTime,
		configMap:  make(map[string]*TailSampling),
	}
}

// hashTraceID 将字符串 TraceID 转换为 uint64
func hashTraceID(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func (s *GlobalSampler) Ingest(packet *TraceDataPacket) {
	// 1. 路由到对应的 Shard
	shard := s.shards[packet.TraceIdHash%uint64(s.shardCount)]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 懒加载初始化
	if shard.activeMap == nil {
		shard.activeMap = make(map[uint64]*TraceData)
		for i := 0; i < 3600; i++ {
			shard.slots[i] = list.New()
		}
	}

	// 2. 获取配置
	config := s.GetConfig(packet.Token)
	if config == nil {
		return // 或者根据业务逻辑直接导出/丢弃
	}

	ttlSec := int(config.TraceTTL.Seconds())
	if ttlSec <= 0 {
		ttlSec = 1
	}
	if ttlSec >= 3600 {
		ttlSec = 3599
	}

	// 计算时间轮槽位
	expirePos := (shard.currentPos + ttlSec) % 3600

	if old, exists := shard.activeMap[packet.TraceIdHash]; exists {
		// --- 场景 A：老 Trace 更新 ---
		// 合并 Span 数据 (packet.Spans 是 proto 生成的 []*point.PBPoint)
		old.td.Spans = append(old.td.Spans, packet.Spans...)
		old.td.HasError = old.td.HasError || packet.HasError

		// 时间轮迁移：从旧格子移到新格子
		shard.slots[old.slotIndex].Remove(old.element)
		old.slotIndex = expirePos
		old.element = shard.slots[expirePos].PushBack(packet.TraceIdHash)
		old.ExpiredTime = time.Now().Unix() + int64(ttlSec)

	} else {
		// --- 场景 B：新 Trace 到达 ---
		// 从 Pool 中获取对象
		td := tracePool.Get().(*TraceData)
		td.td = packet // 直接引用解析好的 proto 对象

		td.slotIndex = expirePos
		td.ExpiredTime = time.Now().Unix() + int64(ttlSec)
		// 挂载到时间轮
		td.element = shard.slots[expirePos].PushBack(packet.TraceIdHash)

		shard.activeMap[packet.TraceIdHash] = td
	}
}

// AdvanceTime 拨动时间轮，返回当前槽位到期的数据
func (s *GlobalSampler) AdvanceTime() map[uint64]*TraceData {
	frozenMap := make(map[uint64]*TraceData)

	for _, shard := range s.shards {
		shard.mu.Lock()

		// 1. 指针向前跳一格
		shard.currentPos = (shard.currentPos + 1) % 3600

		// 2. 获取当前格子的链表
		currList := shard.slots[shard.currentPos]

		// 3. 遍历链表，这里面的全是这一秒该过期的
		for e := currList.Front(); e != nil; {
			next := e.Next()
			tid := e.Value.(uint64)

			if t, ok := shard.activeMap[tid]; ok {
				// 提取数据
				frozenMap[tid] = t
				// 从 Map 中删除
				delete(shard.activeMap, tid)
			}

			// 从链表删除
			currList.Remove(e)
			e = next
		}

		shard.mu.Unlock()
	}
	return frozenMap
}

func (s *GlobalSampler) TailSamplingTraces(traceDatas map[uint64]*TraceData) {
	for _, td := range traceDatas {
		config := s.GetConfig(td.td.Token)
		if tailSampe(td.td, config) {
			// 通过采样规则，确定要保留数据的 发送到中心存储
			// todo ...
		}

		td.Reset()
		tracePool.Put(td)
	}
}

func tailSampe(traceDataPacket *TraceDataPacket, config *TailSampling) bool {
	// todo
	return true
}

func (s *GlobalSampler) UpdateConfig(token string, ts *TailSampling) {
	s.lock.Lock()
	if ts.TraceTTL == 0 {
		ts.TraceTTL = s.waitTime // 使用默认时间
	}

	if tsO, ok := s.configMap[token]; !ok {
		s.configMap[token] = ts
	} else {
		if tsO.Version != ts.Version {
			s.configMap[token] = ts
		}
	}

	s.lock.Unlock()
}

func (s *GlobalSampler) GetConfig(token string) *TailSampling {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.configMap[token]
}

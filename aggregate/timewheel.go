package aggregate

import (
	"container/list"
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/cespare/xxhash/v2"
)

var dataGroupPool = sync.Pool{
	New: func() interface{} {
		return &DataGroup{}
	},
}

// 1. 定义 DataGroup 结构.
type DataGroup struct {
	dataType    string
	td          *DataPacket
	FirstSeen   time.Time
	ExpiredTime int64
	slotIndex   int
	element     *list.Element
}

// Reset 清理函数.
func (dg *DataGroup) Reset() {
	dg.td = nil // 断开对网络包的引用，方便 GC
	dg.element = nil
	dg.slotIndex = 0
	dg.ExpiredTime = 0
}

// 2. 定义分段桶 (Shard).
type Shard struct {
	mu        sync.Mutex
	activeMap map[uint64]*DataGroup

	// 时间轮：本质是一个环形数组
	// 假设最大支持 3600 秒（1小时）的过期时间
	slots      [3600]*list.List
	currentPos int // 当前指针指向的槽位下标
}

// 3. 全局管理器.
type GlobalSampler struct {
	shards     []*Shard
	shardCount int
	waitTime   time.Duration // 5分钟
	configMap  map[string]*TailSamplingConfigs
	lock       sync.RWMutex
}

type TailSamplingOutcome struct {
	Packet       *DataPacket
	SourcePacket *DataPacket
	Decision     DerivedMetricDecision
}

func NewGlobalSampler(shardCount int, waitTime time.Duration) *GlobalSampler {
	sampler := &GlobalSampler{
		shards:     make([]*Shard, shardCount),
		shardCount: shardCount,
		waitTime:   waitTime,
		configMap:  make(map[string]*TailSamplingConfigs),
	}

	for i := 0; i < shardCount; i++ {
		// 1. 初始化 Shard 结构体
		sampler.shards[i] = &Shard{
			activeMap: make(map[uint64]*DataGroup),
			// currentPos 默认为 0
		}

		// 2. 初始化时间轮的 3600 个槽位
		// 必须为每个槽位创建一个新的 list.List
		for j := 0; j < 3600; j++ {
			sampler.shards[i].slots[j] = list.New()
		}
	}

	return sampler
}

func tailSamplingGroupMapKey(packet *DataPacket) uint64 {
	key := HashToken(packet.Token, packet.GroupIdHash)
	key = HashCombine(key, xxhash.Sum64(cliutils.ToUnsafeBytes(packet.DataType)))
	key = HashCombine(key, xxhash.Sum64(cliutils.ToUnsafeBytes(packet.GroupKey)))
	return key
}

func (s *GlobalSampler) Ingest(packet *DataPacket) {
	// 1. 路由到对应的 Shard
	shard := s.shards[packet.GroupIdHash%uint64(s.shardCount)]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 懒加载初始化
	if shard.activeMap == nil {
		shard.activeMap = make(map[uint64]*DataGroup)
		for i := 0; i < 3600; i++ {
			shard.slots[i] = list.New()
		}
	}

	// 2. 获取配置
	var ttlSec int


	switch packet.DataType {
	case point.STracing:
		traceConfig := s.GetTraceConfig(packet.Token)
		if traceConfig == nil {
			l.Errorf("no tail sampling config for token: %s, data type: %s", packet.Token, packet.DataType)
			return
		}

		ttlSec = int(traceConfig.DataTTL.Seconds())
	case point.SLogging:
		loggingConfig := s.GetLoggingConfig(packet.Token)
		if loggingConfig == nil {
			l.Errorf("no tail sampling config for token: %s, data type: %s", packet.Token, packet.DataType)
			return
		}

		ttlSec = int(loggingConfig.DataTTL.Seconds())
	case point.SRUM:
		rumConfig := s.GetRUMConfig(packet.Token)
		if rumConfig == nil {
			l.Errorf("no tail sampling config for token: %s, data type: %s", packet.Token, packet.DataType)
			return
		}
		ttlSec = int(rumConfig.DataTTL.Seconds())
	default:
		l.Errorf("unsupported data type: %s", packet.DataType)
		return
	}

	if ttlSec <= 0 {
		l.Errorf("invalid ttl for data type: %s", packet.DataType)
		return
	}
	if ttlSec >= 3600 {
		ttlSec = 3599
	}
	l.Debugf("ttl is %d for data type: %s", ttlSec, packet.DataType)
	// 计算时间轮槽位
	expirePos := (shard.currentPos + ttlSec) % 3600
	l.Debugf("expirePos is %d", expirePos)

	// 创建组合键
	key := tailSamplingGroupMapKey(packet)

	if old, exists := shard.activeMap[key]; exists {
		// --- 场景 A：老 Trace 更新 ---
		// 合并 Span 数据 (packet.Spans 是 proto 生成的 []*point.PBPoint)
		old.td.Points = append(old.td.Points, packet.Points...)
		old.td.HasError = old.td.HasError || packet.HasError
		old.td.PointCount += packet.PointCount

		if packet.TraceStartTimeUnixNano < old.td.TraceStartTimeUnixNano {
			old.td.TraceStartTimeUnixNano = packet.TraceStartTimeUnixNano
		}
		if packet.TraceEndTimeUnixNano > old.td.TraceEndTimeUnixNano {
			old.td.TraceEndTimeUnixNano = packet.TraceEndTimeUnixNano
		}

		// 时间轮迁移：从旧格子移到新格子
		shard.slots[old.slotIndex].Remove(old.element)
		old.slotIndex = expirePos
		old.element = shard.slots[expirePos].PushBack(key)
		old.ExpiredTime = time.Now().Unix() + int64(ttlSec)
	} else {
		// --- 场景 B：新数据到达 ---
		// 从 Pool 中获取对象
		dg := dataGroupPool.Get().(*DataGroup) //nolint:forcetypeassert
		dg.dataType = packet.DataType
		dg.td = packet // 直接引用解析好的 proto 对象

		dg.slotIndex = expirePos
		dg.ExpiredTime = time.Now().Unix() + int64(ttlSec)
		// 挂载到时间轮
		dg.element = shard.slots[expirePos].PushBack(key)

		shard.activeMap[key] = dg
	}
}

// AdvanceTime 拨动时间轮，返回当前槽位到期的数据.
func (s *GlobalSampler) AdvanceTime() map[uint64]*DataGroup {
	frozenMap := make(map[uint64]*DataGroup)

	for _, shard := range s.shards {
		shard.mu.Lock()

		// 1. 指针向前跳一格
		shard.currentPos = (shard.currentPos + 1) % 3600

		// 2. 获取当前格子的链表
		currList := shard.slots[shard.currentPos]

		// 3. 遍历链表，这里面的全是这一秒该过期的
		for e := currList.Front(); e != nil; {
			next := e.Next()
			key := e.Value.(uint64) //nolint:forcetypeassert

			if dg, ok := shard.activeMap[key]; ok {
				// 提取数据
				frozenMap[key] = dg
				// 从 Map 中删除
				delete(shard.activeMap, key)
			}

			// 从链表删除
			currList.Remove(e)
			e = next
		}

		shard.mu.Unlock()
	}
	return frozenMap
}

func (s *GlobalSampler) TailSamplingOutcomes(dataGroups map[uint64]*DataGroup) map[uint64]*TailSamplingOutcome {
	outcomes := make(map[uint64]*TailSamplingOutcome, len(dataGroups))
	for key, dg := range dataGroups {
		decision := DerivedMetricDecisionDropped
		var keptPacket *DataPacket
		matchedPipeline := false

		switch dg.dataType {
		case point.STracing:
			config := s.GetTraceConfig(dg.td.Token)
			if config == nil {
				l.Infof("tail sampling drop trace: missing trace config, token=%s, group_id=%s",
					dg.td.Token, dg.td.RawGroupId)
			} else {
				for _, pipeline := range config.Pipelines {
					match, packet := pipeline.DoAction(dg.td)
					if match {
						matchedPipeline = true
						// 匹配到了规则
						if packet != nil {
							l.Debugf("matched trace, traceId: %s", packet.RawGroupId)
							keptPacket = packet
							decision = DerivedMetricDecisionKept
						} else {
							l.Infof("tail sampling drop trace by pipeline=%q, token=%s, group_id=%s",
								pipeline.Name, dg.td.Token, dg.td.RawGroupId)
						}
						break
					}
				}
				if !matchedPipeline {
					l.Infof("tail sampling drop trace: no pipeline matched, token=%s, group_id=%s",
						dg.td.Token, dg.td.RawGroupId)
				}
				if len(config.DerivedMetrics) > 0 {
					l.Debugf("custom derived metrics for %s are TODO, token=%s", dg.dataType, dg.td.Token)
				}
			}
		case point.SLogging:
			config := s.GetLoggingConfig(dg.td.Token)
			if config == nil {
				l.Infof("tail sampling drop logging: missing logging config, token=%s, group_id=%s, group_key=%s",
					dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
			} else {
				// 查找对应的分组维度配置
				groupMatched := false
				for _, groupDim := range config.GroupDimensions {
					if groupDim.GroupKey == dg.td.GroupKey {
						groupMatched = true
						for _, pipeline := range groupDim.Pipelines {
							match, packet := pipeline.DoAction(dg.td)
							if match {
								matchedPipeline = true
								// 匹配到了规则
								if packet != nil {
									l.Debugf("matched logging, groupId: %s", packet.RawGroupId)
									keptPacket = packet
									decision = DerivedMetricDecisionKept
								} else {
									l.Infof("tail sampling drop logging by pipeline=%q, token=%s, group_id=%s, group_key=%s",
										pipeline.Name, dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
								}
								break
							}
						}
						if !matchedPipeline {
							l.Infof("tail sampling drop logging: no pipeline matched, token=%s, group_id=%s, group_key=%s",
								dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
						}
						if len(groupDim.DerivedMetrics) > 0 {
							l.Debugf("custom derived metrics for %s are TODO, token=%s, group_key=%s", dg.dataType, dg.td.Token, groupDim.GroupKey)
						}
						break
					}
				}
				if !groupMatched {
					l.Infof("tail sampling drop logging: no group dimension matched, token=%s, group_id=%s, group_key=%s",
						dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
				}
			}
		case point.SRUM:
			config := s.GetRUMConfig(dg.td.Token)
			if config == nil {
				l.Infof("tail sampling drop rum: missing rum config, token=%s, group_id=%s, group_key=%s",
					dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
			} else {
				// 查找对应的分组维度配置
				groupMatched := false
				for _, groupDim := range config.GroupDimensions {
					if groupDim.GroupKey == dg.td.GroupKey {
						groupMatched = true
						for _, pipeline := range groupDim.Pipelines {
							match, packet := pipeline.DoAction(dg.td)
							if match {
								matchedPipeline = true
								// 匹配到了规则
								if packet != nil {
									l.Debugf("matched RUM, groupId: %s", packet.RawGroupId)
									keptPacket = packet
									decision = DerivedMetricDecisionKept
								} else {
									l.Infof("tail sampling drop rum by pipeline=%q, token=%s, group_id=%s, group_key=%s",
										pipeline.Name, dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
								}
								break
							}
						}
						if !matchedPipeline {
							l.Infof("tail sampling drop rum: no pipeline matched, token=%s, group_id=%s, group_key=%s",
								dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
						}
						if len(groupDim.DerivedMetrics) > 0 {
							l.Debugf("custom derived metrics for %s are TODO, token=%s, group_key=%s", dg.dataType, dg.td.Token, groupDim.GroupKey)
						}
						break
					}
				}
				if !groupMatched {
					l.Infof("tail sampling drop rum: no group dimension matched, token=%s, group_id=%s, group_key=%s",
						dg.td.Token, dg.td.RawGroupId, dg.td.GroupKey)
				}
			}
		default:
			l.Errorf("unsupported data type in tail sampling: %s", dg.dataType)
		}

		l.Infof("tail sampling outcome: decision=%s, token=%s, data_type=%s, group_id=%s, group_key=%s, kept=%t",
			decision, dg.td.Token, dg.td.DataType, dg.td.RawGroupId, dg.td.GroupKey, keptPacket != nil)

		outcomes[key] = &TailSamplingOutcome{
			Packet:       keptPacket,
			SourcePacket: dg.td,
			Decision:     decision,
		}

		dg.Reset()
		dataGroupPool.Put(dg)
	}
	return outcomes
}

func (s *GlobalSampler) TailSamplingData(dataGroups map[uint64]*DataGroup) map[uint64]*DataPacket {
	outcomes := s.TailSamplingOutcomes(dataGroups)
	packets := make(map[uint64]*DataPacket)

	for key, outcome := range outcomes {
		if outcome == nil || outcome.Packet == nil {
			continue
		}
		packets[key] = outcome.Packet
	}

	return packets
}

func (s *GlobalSampler) UpdateConfig(token string, ts *TailSamplingConfigs) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if ts == nil {
		return nil
	}
	// 设置各数据类型的默认 TTL
	if ts.Tracing != nil && ts.Tracing.DataTTL == 0 {
		ts.Tracing.DataTTL = s.waitTime
	}
	if ts.Logging != nil && ts.Logging.DataTTL == 0 {
		ts.Logging.DataTTL = s.waitTime
	}
	if ts.RUM != nil && ts.RUM.DataTTL == 0 {
		ts.RUM.DataTTL = s.waitTime
	}

	if tsO, ok := s.configMap[token]; !ok {
		if err := ts.Init(); err != nil {
			return err
		}
		s.configMap[token] = ts
	} else if tsO.Version != ts.Version {
		if err := ts.Init(); err != nil {
			return err
		}
		s.configMap[token] = ts
	}

	return nil
}

func (s *GlobalSampler) GetTraceConfig(token string) *TraceTailSampling {
	s.lock.RLock()
	defer s.lock.RUnlock()
	config, ok := s.configMap[token]
	if !ok {
		return nil
	}
	return config.Tracing
}

func (s *GlobalSampler) GetLoggingConfig(token string) *LoggingTailSampling {
	s.lock.RLock()
	defer s.lock.RUnlock()
	config, ok := s.configMap[token]
	if !ok {
		return nil
	}
	return config.Logging
}

func (s *GlobalSampler) GetRUMConfig(token string) *RUMTailSampling {
	s.lock.RLock()
	defer s.lock.RUnlock()
	config, ok := s.configMap[token]
	if !ok {
		return nil
	}
	return config.RUM
}

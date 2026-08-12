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

// DataGroup 是时间轮中的 entry， 仅持有原始 point 二进制切片，避免长期持有展开对象图。
type DataGroup struct {
	dataType    string
	packet      *DataPacket
	FirstSeen   time.Time
	ExpiredTime int64
	slotIndex   int
	element     *list.Element
	spillKey    string // 大包 payload 落盘后的 key（空表示在内存）
}

// Reset 清理函数.
func (dg *DataGroup) Reset() {
	dg.dataType = ""
	dg.packet = nil
	dg.FirstSeen = time.Time{}
	dg.element = nil
	dg.slotIndex = 0
	dg.ExpiredTime = 0
	dg.spillKey = ""
}

// Shard 定义分段桶.
type Shard struct {
	mu        sync.Mutex
	activeMap map[uint64]*DataGroup

	// 时间轮：本质是一个环形数组
	// 假设最大支持 3600 秒（1小时）的过期时间
	slots      [3600]*list.List
	currentPos int // 当前指针指向的槽位下标
}

// GlobalSampler 全局管理器.
type GlobalSampler struct {
	shards     []*Shard
	shardCount int
	waitTime   time.Duration // 5分钟
	configMap  map[string]*TailSamplingConfigs
	lock       sync.RWMutex

	// 大包分级落盘（可选，默认不启用）：
	// 超过 spillThreshold 的 payload 写入 spiller，时间轮只保留元数据；
	// 决策需要时惰性读回，决策完成后立即释放。
	spiller        PayloadSpiller
	spillThreshold int
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

// SetPayloadSpiller 启用大包分级落盘。
// threshold 为落盘阈值（字节），<=0 或 spiller 为 nil 时不启用。
func (s *GlobalSampler) SetPayloadSpiller(spiller PayloadSpiller, threshold int) {
	if s == nil {
		return
	}

	if spiller == nil || threshold <= 0 {
		s.spiller = nil
		s.spillThreshold = 0
		return
	}

	s.spiller = spiller
	s.spillThreshold = threshold
}

// spillIfNeeded 将超过阈值的 payload 落盘并清空内存字节；
// 返回落盘 key 与是否实际落盘。
func (s *GlobalSampler) spillIfNeeded(packet *DataPacket) (string, bool) {
	if s == nil || s.spiller == nil || packet == nil {
		return "", false
	}
	if len(packet.PointsPayload) <= s.spillThreshold {
		return "", false
	}

	key, err := s.spiller.Put(packet.PointsPayload)
	if err != nil {
		l.Errorf("spill packet payload failed: %v", err)
		return "", false
	}

	packet.PointsPayload = nil
	return key, true
}

// hydrateDataGroup 读回 spill 的 payload 到内存。
func (s *GlobalSampler) hydrateDataGroup(dg *DataGroup) error {
	if dg == nil || dg.spillKey == "" || s.spiller == nil {
		return nil
	}

	payload, err := s.spiller.Get(dg.spillKey)
	if err != nil {
		return err
	}

	dg.packet.PointsPayload = payload
	dg.spillKey = ""
	return nil
}

// releaseSpill 删除 spill 数据（幂等）。
func (s *GlobalSampler) releaseSpill(key string) {
	if s == nil || s.spiller == nil || key == "" {
		return
	}

	if err := s.spiller.Delete(key); err != nil {
		l.Errorf("release spill payload failed: %v", err)
	}
}

// needsPayloadForDecision 判断该 token/类型/分组的决策是否需要 payload 内容。
// 全部 pipeline 可被谓词摘要覆盖（或为 match-all 概率采样）时无需 payload。
func (s *GlobalSampler) needsPayloadForDecision(dataType, token, groupKey string) bool {
	if s == nil {
		return true
	}

	pipelines := s.pipelinesFor(dataType, token, groupKey)
	_, ok := compilePipelinesFast(pipelines)
	return !ok
}

func tailSamplingGroupMapKey(packet *DataPacket) uint64 {
	if packet == nil {
		return 0
	}

	return tailSamplingGroupMapKeyByFields(packet.Token, packet.GroupIdHash, packet.DataType, packet.GroupKey)
}

func tailSamplingGroupMapKeyByFields(token string, groupIDHash uint64, dataType, groupKey string) uint64 {
	key := HashToken(token, groupIDHash)
	key = HashCombine(key, xxhash.Sum64(cliutils.ToUnsafeBytes(dataType)))
	key = HashCombine(key, xxhash.Sum64(cliutils.ToUnsafeBytes(groupKey)))
	return key
}

// mergePacketPayload 将 src 的 points payload 合并进 dst。
// 两边都未压缩时保持直接拼接路径；任一压缩时解压后合并并重新压缩。
func mergePacketPayload(dst, src *DataPacket) error {
	if dst == nil || src == nil {
		return nil
	}

	if dst.PayloadCompression == PayloadCompressionNone && src.PayloadCompression == PayloadCompressionNone {
		dst.PointsPayload = append(dst.PointsPayload, src.PointsPayload...)
		return nil
	}

	dstPayload, err := DecompressPointsPayload(dst.PointsPayload, dst.PayloadCompression)
	if err != nil {
		return err
	}
	srcPayload, err := DecompressPointsPayload(src.PointsPayload, src.PayloadCompression)
	if err != nil {
		return err
	}

	merged := make([]byte, 0, len(dstPayload)+len(srcPayload))
	merged = append(merged, dstPayload...)
	merged = append(merged, srcPayload...)

	compressed, compression, err := CompressPointsPayload(merged)
	if err != nil {
		return err
	}

	dst.PointsPayload = compressed
	dst.PayloadCompression = compression
	return nil
}

// mergeSpanPredicates 将 src 的 span 谓词摘要 OR 合并进 dst。
// 布尔谓词取或；duration 摘要取最大值。
// 跨 Datakit 场景：同一 trace 的 span 分散在多个 Datakit，
// 合并后谓词即为"所有已见 Datakit 的并集"，与决策时 walk 完整 payload 的结果一致。
func mergeSpanPredicates(dst, src *DataPacket) {
	if dst == nil || src == nil {
		return
	}

	dst.PredError = dst.PredError || src.PredError
	dst.PredHttpError = dst.PredHttpError || src.PredHttpError
	dst.PredBizError = dst.PredBizError || src.PredBizError
	dst.PredTraceKeep = dst.PredTraceKeep || src.PredTraceKeep

	if src.MaxSpanDurationUs > dst.MaxSpanDurationUs {
		dst.MaxSpanDurationUs = src.MaxSpanDurationUs
	}
	if src.RootDurationUs > dst.RootDurationUs {
		dst.RootDurationUs = src.RootDurationUs
	}
	if src.MaxNonrootDurationUs > dst.MaxNonrootDurationUs {
		dst.MaxNonrootDurationUs = src.MaxNonrootDurationUs
	}
}

func (s *GlobalSampler) Ingest(packet *DataPacket) {
	if s == nil || packet == nil || s.shardCount == 0 {
		return
	}

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
	// 计算时间轮槽位
	expirePos := (shard.currentPos + ttlSec) % 3600

	// 创建组合键
	key := tailSamplingGroupMapKey(packet)

	pointCount := packetPointCount(packet)

	if old, exists := shard.activeMap[key]; exists {
		// --- 场景 A：老分组更新 ---
		oldSpillKey := old.spillKey
		if oldSpillKey != "" {
			if err := s.hydrateDataGroup(old); err != nil {
				l.Errorf("hydrate spill payload for merge failed: %v", err)
			}
		}

		if err := mergePacketPayload(old.packet, packet); err != nil {
			l.Errorf("merge packet payload failed: %v", err)
			return
		}
		old.packet.HasError = old.packet.HasError || packet.HasError
		mergeSpanPredicates(old.packet, packet)
		old.packet.PointCount += pointCount

		if packet.TraceStartTimeUnixNano > 0 {
			if old.packet.TraceStartTimeUnixNano == 0 || packet.TraceStartTimeUnixNano < old.packet.TraceStartTimeUnixNano {
				old.packet.TraceStartTimeUnixNano = packet.TraceStartTimeUnixNano
			}
		}
		if packet.TraceEndTimeUnixNano > old.packet.TraceEndTimeUnixNano {
			old.packet.TraceEndTimeUnixNano = packet.TraceEndTimeUnixNano
		}
		if packet.MaxPointTimeUnixNano > old.packet.MaxPointTimeUnixNano {
			old.packet.MaxPointTimeUnixNano = packet.MaxPointTimeUnixNano
		}
		if old.packet.RawGroupId == "" {
			old.packet.RawGroupId = packet.RawGroupId
		}
		if packet.ConfigVersion > old.packet.ConfigVersion {
			old.packet.ConfigVersion = packet.ConfigVersion
		}
		if old.packet.Source == "" {
			old.packet.Source = packet.Source
		}

		// 合并后重新判定是否落盘，并释放合并前的磁盘数据。
		if key, spilled := s.spillIfNeeded(old.packet); spilled {
			old.spillKey = key
		} else {
			old.spillKey = ""
		}
		s.releaseSpill(oldSpillKey)

		// 时间轮迁移：从旧格子移到新格子
		if old.element != nil {
			shard.slots[old.slotIndex].Remove(old.element)
		}
		old.slotIndex = expirePos
		old.element = shard.slots[expirePos].PushBack(key)
		old.ExpiredTime = time.Now().Unix() + int64(ttlSec)
	} else {
		// --- 场景 B：新数据到达 ---
		// 从 Pool 中获取对象
		dg := dataGroupPool.Get().(*DataGroup) //nolint:forcetypeassert
		dg.dataType = packet.DataType
		dg.packet = packet
		if dg.packet.PointCount == 0 {
			dg.packet.PointCount = pointCount
		}
		dg.FirstSeen = time.Now()

		dg.slotIndex = expirePos
		dg.ExpiredTime = time.Now().Unix() + int64(ttlSec)
		// 大包落盘：时间轮只保留元数据
		if key, spilled := s.spillIfNeeded(packet); spilled {
			dg.spillKey = key
		}
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
		if dg == nil || dg.packet == nil {
			outcomes[key] = &TailSamplingOutcome{Decision: DerivedMetricDecisionDropped}
			continue
		}

		sourcePacket := dg.packet

		// 落盘的包：决策需要 payload 内容（pipeline 无法被谓词摘要覆盖），
		// 或谓词摘要不可信（全零，旧数据/手工构造）时读回；决策完成后统一释放。
		spillKey := dg.spillKey
		if spillKey != "" && (s.needsPayloadForDecision(dg.dataType, sourcePacket.Token, sourcePacket.GroupKey) ||
			!packetHasSpanPredicates(sourcePacket)) {
			if err := s.hydrateDataGroup(dg); err != nil {
				l.Errorf("hydrate spill payload for decision failed: %v", err)
			}
		}

		decision := DerivedMetricDecisionDropped
		var keptPacket *DataPacket

		token := dg.packet.Token
		groupKey := dg.packet.GroupKey

		pipelines := s.pipelinesFor(dg.dataType, token, groupKey)
		if pipelines != nil && sourcePacket != nil {
			// spill 包且 pipeline 可编译、谓词可信：直接谓词判定，避免读盘；
			// 其余情况走 evaluatePipelines（内部快速路径或解压 walk）。
			usePredicates := spillKey != "" &&
				!s.needsPayloadForDecision(dg.dataType, token, groupKey) &&
				packetHasSpanPredicates(sourcePacket)
			if usePredicates {
				if match, packet := decideWithPredicates(sourcePacket, pipelines); match && packet != nil {
					keptPacket = packet
					decision = DerivedMetricDecisionKept
				}
			} else {
				if match, packet := evaluatePipelines(sourcePacket, pipelines); match && packet != nil {
					keptPacket = packet
					decision = DerivedMetricDecisionKept
				}
			}
		}

		// kept 包后续要发送给下游，必须持有完整 payload：决策未读回时此处补读。
		if spillKey != "" && keptPacket != nil && len(keptPacket.PointsPayload) == 0 {
			if err := s.hydrateDataGroup(dg); err != nil {
				l.Errorf("hydrate spill payload for kept packet failed: %v", err)
			}
		}
		// 决策完成，释放磁盘数据（无论 kept/dropped，payload 已读回或已丢弃）。
		s.releaseSpill(spillKey)

		outcomes[key] = &TailSamplingOutcome{
			Packet:       keptPacket,
			SourcePacket: sourcePacket,
			Decision:     decision,
		}

		dg.Reset()
		dataGroupPool.Put(dg)
	}
	return outcomes
}

// pipelinesFor 按数据类型/分组维度返回采样 pipeline 配置。
func (s *GlobalSampler) pipelinesFor(dataType, token, groupKey string) []*SamplingPipeline {
	switch dataType {
	case point.STracing:
		if cfg := s.GetTraceConfig(token); cfg != nil {
			return cfg.Pipelines
		}
	case point.SLogging:
		if cfg := s.GetLoggingConfig(token); cfg != nil {
			for _, group := range cfg.GroupDimensions {
				if group != nil && group.GroupKey == groupKey {
					return group.Pipelines
				}
			}
		}
	case point.SRUM:
		if cfg := s.GetRUMConfig(token); cfg != nil {
			for _, group := range cfg.GroupDimensions {
				if group != nil && group.GroupKey == groupKey {
					return group.Pipelines
				}
			}
		}
	}
	return nil
}

// decideWithPredicates 用 span 谓词摘要判定 pipelines（不读 payload）。
// pipelines 必须全部可编译，否则返回不匹配。
func decideWithPredicates(sourcePacket *DataPacket, pipelines []*SamplingPipeline) (bool, *DataPacket) {
	exprs, ok := compilePipelinesFast(pipelines)
	if !ok {
		return false, nil
	}

	for idx, expr := range exprs {
		if expr == nil {
			continue
		}
		if expr(sourcePacket) {
			return true, pipelineMatchedPacket(sourcePacket, pipelines[idx])
		}
	}

	return false, nil
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

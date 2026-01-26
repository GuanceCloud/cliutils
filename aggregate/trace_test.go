package aggregate

import (
	"container/list"
	"github.com/stretchr/testify/assert" // 建议使用 assert 库增加可读性
	"testing"
	"time"
)

func TestGlobalSampler_AdvanceTime(t *testing.T) {
	// 1. 初始化采样器
	shardCount := 4
	sampler := &GlobalSampler{
		shardCount: shardCount,
		shards:     make([]*Shard, shardCount),
		configMap: map[string]*TailSampling{
			"TokenA": {
				TraceTTL:       time.Second * 2,
				DerivedMetrics: nil,
				Pipelines:      nil,
				Version:        0,
			},
		},
	}
	for i := 0; i < shardCount; i++ {
		sampler.shards[i] = &Shard{
			activeMap: make(map[uint64]*TraceData),
		}
		for j := 0; j < 3600; j++ {
			sampler.shards[i].slots[j] = list.New()
		}
	}

	// 模拟一条数据：TokenA, TTL=2s, TraceIDHash=100
	tidHash := uint64(100)
	packet := &TraceDataPacket{
		TraceIdHash: tidHash,
		Token:       "TokenA",
	}

	// 这里我们需要模拟 GetConfig 返回 2s 的 TTL
	// 假设我们在 Ingest 内部已经根据当前指针计算好了位置
	// 当前指针为 0，TTL 为 2，预期的过期槽位是 2
	sampler.Ingest(packet)

	// --- 第一次拨动：从 0 到 1 ---
	expired := sampler.AdvanceTime()
	assert.Equal(t, 0, len(expired), "在第 1 秒不应该有数据过期")

	// --- 第二次拨动：从 1 到 2 ---
	expired = sampler.AdvanceTime()
	assert.Equal(t, 1, len(expired), "在第 2 秒应该拿到过期数据")
	assert.NotNil(t, expired[tidHash])
	t.Logf("data :=%v", expired[tidHash])
	assert.Equal(t, tidHash, expired[tidHash].td.TraceIdHash)

	// 检查 activeMap 是否已清理
	shard := sampler.shards[tidHash%uint64(shardCount)]
	_, exists := shard.activeMap[tidHash]
	assert.False(t, exists, "过期后数据应从 activeMap 中删除")
}

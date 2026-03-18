package aggregate

import (
	"container/list"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestGlobalSampler_AdvanceTime(t *testing.T) { //nolint
	// 1. 初始化采样器
	shardCount := 4
	sampler := &GlobalSampler{
		shardCount: shardCount,
		shards:     make([]*Shard, shardCount),
		configMap: map[string]*TailSamplingConfigs{
			"TokenA": {
				Tracing: &TraceTailSampling{
					DataTTL:        time.Second * 2,
					DerivedMetrics: nil,
					Pipelines:      nil,
					Version:        0,
				},
			},
		},
	}
	for i := 0; i < shardCount; i++ {
		sampler.shards[i] = &Shard{
			activeMap: make(map[uint64]*DataGroup),
		}
		for j := 0; j < 3600; j++ {
			sampler.shards[i].slots[j] = list.New()
		}
	}

	// 模拟一条数据：TokenA, TTL=2s, TraceIDHash=100
	tidHash := uint64(100)
	packet := &DataPacket{
		GroupIdHash: tidHash,
		Token:       "TokenA",
		DataType:    "tracing",
	}
	cKey := HashToken(packet.Token, packet.GroupIdHash)
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
	assert.NotNil(t, expired[cKey])
	t.Logf("data :=%v", expired[cKey])
	assert.Equal(t, tidHash, expired[cKey].td.GroupIdHash)

	// 检查 activeMap 是否已清理
	shard := sampler.shards[tidHash%uint64(shardCount)]
	_, exists := shard.activeMap[cKey]
	assert.False(t, exists, "过期后数据应从 activeMap 中删除")
}

type MockSampler struct {
	sampler *GlobalSampler
	t       *testing.T
	stop    chan struct{}
}

func (m *MockSampler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		m.t.Errorf("parse query failed:%v", err)
		w.WriteHeader(400)
		w.Write([]byte("bad query"))
	}
	token := values.Get("token")
	m.t.Logf("token:%s", token)
	bts, err := io.ReadAll(r.Body)
	if err != nil {
		m.t.Errorf("read body failed:%v", err)
		w.Write([]byte("bad body"))
		return
	}
	batch := &DataPacket{}
	err = proto.Unmarshal(bts, batch)
	if err != nil {
		m.t.Errorf("unmarshal failed:%v", err)
		w.Write([]byte("bad body"))
		return
	}
	batch.Token = token
	m.sampler.Ingest(batch)
}

func (m *MockSampler) getTrace() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			traces := m.sampler.AdvanceTime()
			if len(traces) > 0 {
				for _, trace := range traces {
					if trace.td != nil {
						m.t.Logf("trace:%v", trace.td.RawGroupId)
						for _, span := range trace.td.Points {
							m.t.Logf("span:%v", span.String())
						}
					}
				}
			} else {
				m.t.Logf("no trace")
			}
		case <-m.stop:
			return
		}
	}
}

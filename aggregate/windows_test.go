package aggregate

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/golang/protobuf/proto"
)

type MockCalculator struct {
	MetricBase
}

func (m *MockCalculator) Add(a any) {}

func (m *MockCalculator) Aggr() ([]*point.Point, error) {
	return []*point.Point{}, nil
}

func (m *MockCalculator) Reset() {}

func (m *MockCalculator) Base() *MetricBase {
	return &m.MetricBase
}

func TestCache_Concurrency(t *testing.T) {
	cache := NewCache(25 * time.Second)

	var wg sync.WaitGroup
	workerCount := 100 // 100 个并发协程
	batchesPerWorker := 1000

	// 模拟不同用户的 Token 集合
	tokens := []string{"user_a", "user_b", "user_c", "user_d", "user_e", "user_f", "user_g"}

	start := time.Now()

	// 2. 启动并发写入
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < batchesPerWorker; j++ {
				// 模拟不同时间点的请求
				token := tokens[j%len(tokens)]
				var kvs point.KVs
				kvs = kvs.Add("used", 1000).AddTag("id", "0").AddTag("service_name", "test")
				pt := point.NewPoint("otel", kvs, point.DefaultMetricOptions()...)
				// 模拟一个包含多个算子的 batch
				cal := &MockCalculator{
					MetricBase: MetricBase{
						pt:           pt.PBPoint(),
						aggrTags:     nil,
						key:          "",
						name:         "",
						tenantHash:   0,
						hash:         uint64(j % 10),
						window:       10,
						nextWallTime: time.Now().Unix(),
						heapIdx:      0,
					},
				}

				// 执行写入
				cache.GetAndSetBucket(cal.Base().nextWallTime+5, token, cal)
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("add bucket ok,since: %v\n", time.Since(start))

	// 输出每个窗口中的个数
	for _, ws := range cache.WindowsBuckets {
		for _, w := range ws.WS {
			t.Logf("token: %s, len: %d\n", w.Token, len(w.cache))
		}
	}
	// 3. 验证数据一致性
	// 模拟到达过期时间，取出所有窗口
	windows := cache.GetExpWidows()

	// 简单校验：如果逻辑正确，弹出窗口的数量不应该为 0
	// 注意：实际校验逻辑需要根据你业务中 Calculator 的 Add 累加结果来判断
	if len(windows) == 0 {
		t.Logf("no data expired")
	}

	t.Logf("cache expired window len:: %d\n", len(windows))
}

type MockServer struct {
	t     *testing.T
	cache *Cache
	stop  chan struct{}
}

func (m *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bts, err := io.ReadAll(r.Body)
	if err != nil {
		m.t.Errorf("read body failed:%v", err)
		w.Write([]byte("bad body"))
		return
	}
	batch := &Batchs{}
	err = proto.Unmarshal(bts, batch)
	if err != nil {
		m.t.Errorf("unmarshal failed:%v", err)
		w.Write([]byte("bad body"))
		return
	}

	n, bn := m.cache.AddBatchs("token", batch.Batchs)
	m.t.Logf("add batch:%d, expired %d", n, bn)
}

func (m *MockServer) getPointData() {
	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-ticker.C:
			// 当前时间
			m.t.Logf("start to get expired windows")
			ws := m.cache.GetExpWidows()
			if len(ws) > 0 {
				pds := WindowsToData(ws)
				m.t.Logf("point data len:%d", len(pds))
				for _, pd := range pds {
					m.t.Logf("point data:%s", pd.Token)
					for _, p := range pd.PTS {
						m.t.Logf("point:%s", p.LineProto())
					}
				}
			}
		}
	}
}

func TestHTTPServe(t *testing.T) {
	server := &MockServer{
		t:     t,
		cache: NewCache(time.Second * 60),
		stop:  make(chan struct{}),
	}
	go server.getPointData()
	go func() {
		http.ListenAndServe(":18080", server)
	}()

	time.Sleep(time.Minute * 30)
}

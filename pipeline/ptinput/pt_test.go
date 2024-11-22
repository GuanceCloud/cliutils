package ptinput

import (
	"strconv"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/stretchr/testify/assert"
)

type tcase struct {
	name    string
	tags    map[string]string
	fiellds map[string]any
	time    time.Time

	cat point.Category
}

func (c *tcase) Point() *point.Point {
	kvs := point.NewKVs(c.fiellds)
	for k, v := range c.tags {
		kvs = kvs.MustAddTag(k, v)
	}

	var opt []point.Option

	switch c.cat {
	case point.Metric:
		opt = point.DefaultMetricOptions()
	case point.Object:
		opt = point.DefaultObjectOptions()
	default:
		opt = point.DefaultLoggingOptions()
	}
	opt = append(opt, point.WithTime(c.time))
	return point.NewPointV2(c.name, kvs, opt...)
}

func TestPlPt(t *testing.T) {
	cases := []tcase{
		{
			name: "t1",
			tags: map[string]string{
				"t1": "v1",
				"t2": "v2",
			},
			fiellds: map[string]any{
				"f1": 1,
				"f2": 2,
			},
			cat:  point.Metric,
			time: time.Now(),
		},
	}

	pt := cases[0].Point()

	plpt := PtWrap(cases[0].cat, pt)

	plpt.Set("f1", "f1", ast.String)
	plpt.Set("f3", "v3", ast.String)
	plpt.Set("t2", "v3", ast.String)
}

func TestPlPt2(t *testing.T) {
	pt := point.NewPointV2("t", point.NewKVs(map[string]any{
		"a1":  "1",
		"xx2": "1",
	}), point.WithTime(time.Now()))

	pp := PtWrap(point.Logging, pt)
	if _, _, err := pp.Get("a"); err == nil {
		t.Fatal("err == nil")
	}

	if _, _, e := pp.Get("a"); e == nil {
		t.Fatal("ok")
	}

	if ok := pp.Set("a", 1, ast.Int); !ok {
		t.Fatal(ok)
	}

	if ok := pp.Set("a1", []any{1}, ast.List); !ok {
		t.Fatal(ok)
	}

	if ok := pp.Set("xx2", []any{1}, ast.List); !ok {
		t.Fatal(ok)
	}

	if ok := pp.Set("xx2", 1.2, ast.Float); !ok {
		t.Fatal(ok)
	}

	if v, _, err := pp.Get("xx2"); err != nil {
		assert.Equal(t, 1.2, v)
		t.Fatal(err)
	}

	if err := pp.RenameKey("xx2", "xxb"); err != nil {
		t.Fatal(err)
	}

	if ok := pp.SetTag("a", 1., ast.Float); !ok {
		t.Fatal(ok)
	}

	if ok := pp.Set("a", 1, ast.Int); !ok {
		t.Fatal(ok)
	}

	if _, ok := pp.Fields()["a"]; ok {
		t.Fatal("a in fields")
	}

	if err := pp.RenameKey("a", "b"); err != nil {
		t.Fatal(err)
	}

	if pp.PtTime().UnixNano() == 0 {
		t.Fatal("time == 0")
	}

	pp.GetAggBuckets()
	pp.SetAggBuckets(nil)

	pp.Set("time", 1, ast.Int)
	pp.KeyTime2Time()
	ppt := pp.Point()
	if ppt.Time().UnixNano() != 1 {
		t.Fatal("time != 1")
	}

	pp.MarkDrop(true)
	if !pp.Dropped() {
		t.Fatal("!dropped")
	}

	dpt := pp.Point()

	pp = PtWrap(point.Logging, dpt)

	if _, _, err := pp.Get("b"); err != nil {
		t.Fatal(err.Error())
	}

	if _, ok := pp.Tags()["b"]; !ok {
		t.Fatal("b not in tags")
	}

	if _, dtyp, e := pp.Get("b"); e != nil || dtyp != ast.String {
		t.Fatal("not tag")
	}

	if ok := pp.Set("b", []any{}, ast.List); !ok {
		t.Fatal(ok)
	}

	if _, ok := pp.Fields()["xxb"]; !ok {
		t.Fatal("xxb not in field")
	}

	if pp.GetPtName() != "t" {
		t.Fatal("name != \"t\"")
	}

	pp.SetPtName("t2")
	if pp.GetPtName() != "t2" {
		t.Fatal("name != \"t2\"")
	}
}

var lp = []byte(`gin app="deployment-forethought-kodo-kodo",client_ip="172.1***03",cluster_name_k8s="k8s-daily",container_id="dcbacc667c1534127d4f4c531fc26f613f4e6f822e646dee4e4bdbc5e87920c4",container_name="kodo",deployment="kodo",filepath="/rootfs/var/log/pods/forethought-kodo_kodo-7dc8b5c448-rmcpb_bd5159c7-df57-4346-987d-fc6883aeabea/kodo/0.log",guance_site="daily",host="cluster_a_cn-hangzhou.172.1***.102",host_ip="172.1***.102",image="registry.****.com/ko**:testing-202*****",log_read_lines=289892,message="[GIN] 2024/11/15 - 10:56:07 | 403 | 759.859µs |  172.16.200.203 | POST    \"/v1/write/metric?token=****************842cda605c6cb87e3a7b8\"",message_length=137,namespace="forethought-kodo",pod-template-hash="7dc8b5c448",pod_ip="10.113.0.204",pod_name="kodo-7dc8b5c448-rmcpb",real_host="hz-dataflux-daily-002",region="cn-hangzhou",service="kodo",status="warning",time_ns=1731639367526632400,time_us=1731639367526632,timestamp="2024/11/15 - 10:56:07",zone_id="cn-hangzhou-j" 1731639367526000000`)

func BenchmarkPts(b *testing.B) {
	dec := point.GetDecoder()
	pts, err := dec.Decode([]byte(lp))
	assert.NoError(b, err)
	pt := pts[0]
	pt.KVs().AddV2("message", "", true, point.WithKVTagSet(false))

	b.Run("pt_old_add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pts, _ := dec.Decode(lp)
			pt := pts[0]
			pti := WrapPoint(point.Logging, pt)
			ptMockOperatorAdd(pti)
			pti.Point()
		}
	})

	b.Run("pt_add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pts, _ := dec.Decode(lp)
			pt := pts[0]
			pti := PtWrap(point.Logging, pt)
			ptMockOperatorAdd(pti)
			pti.Point()
		}
	})

	b.Run("pt_old_drop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pts, _ := dec.Decode(lp)
			pt := pts[0]
			pti := WrapPoint(point.Logging, pt)
			ptMockOperatorDrop(pti)
			pti.Point()
		}
	})

	b.Run("pt_drop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pts, _ := dec.Decode(lp)
			pt := pts[0]
			pti := PtWrap(point.Logging, pt)
			ptMockOperatorDrop(pti)
			pti.Point()
		}
	})

	b.Run("pt_old_del", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pts, _ := dec.Decode(lp)
			pt := pts[0]
			pti := WrapPoint(point.Logging, pt)
			ptMockOperatorDel(pti)
			pti.Point()
		}
	})

	b.Run("pt_del", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pts, _ := dec.Decode(lp)
			pt := pts[0]
			pti := PtWrap(point.Logging, pt)
			ptMockOperatorDel(pti)
			pti.Point()
		}
	})
}

func ptMockOperatorAdd(pti PlInputPt) {
	for i := 0; i < 5; i++ {
		pti.Get("_")
	}

	n := "xyz_"
	for i := 0; i < 20; i++ {
		n := n + strconv.Itoa(i)
		pti.Set(n, n, ast.String)
	}
}

func ptMockOperatorDrop(pti PlInputPt) {
	for i := 0; i < 5; i++ {
		pti.Get("_")
	}

	pti.MarkDrop(true)
}

func ptMockOperatorDel(pti PlInputPt) {
	for i := 0; i < 5; i++ {
		pti.Get("_")
	}

	n := "xyz_"
	for i := 0; i < 10; i++ {
		n += strconv.Itoa(i)
		pti.Set(n, n, ast.String)
	}

	for i := 0; i < 5; i++ {
		n += strconv.Itoa(15 - i - 1)
		pti.Set(n, n, ast.String)
	}
}

package luascript

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"
)

func TestScript(t *testing.T) {
	var luaCode = `
function handle(points)
	table.insert(points,
		{
			name="create_name1",
			time=1601285330806946000,
			tags={ t1="create_tags_01", t2="create_tags_02" },
			fields={ f1="create_fields_01", f5=555555, f6=true }
		}
	)
	for _, pt in pairs(points) do
		print("name", pt.name)
		print("time", pt.time)
		print("-------\ntags:")
		for k, v in pairs(pt.tags) do
			print(k, v)
		end
		print("-------\nfields:")
		for k, v in pairs(pt.fields) do
			print(k, v)
		end
		print("-----------------------")
	end
	return points
end
`
	pt1, _ := influxdb.NewPoint("point01",
		map[string]string{
			"t1": "tags10",
			"t2": "tags20",
		},
		map[string]interface{}{
			"f1": uint(11),
			"f2": true,
			"f3": "hello",
		},
		time.Now(),
	)
	pt2, _ := influxdb.NewPoint("point02",
		map[string]string{
			"t1": "tags11",
			"t2": "tags21",
		},
		map[string]interface{}{
			"f1": uint(33),
			"f2": int32(444),
			"f4": "world",
		},
		time.Now(),
	)

	var err error
	err = AddLuaCodes("ptdata", []string{luaCode})
	if err != nil {
		t.Fatal(err)
	}

	Run()

	p, err := NewPointData("ptdata", []*influxdb.Point{pt1, pt2})
	if err != nil {
		t.Fatal(err)
	}

	SendData(p)

	time.Sleep(time.Second * 1)

	Stop()
}

type pointData struct {
	name   string
	data   []map[string]interface{}
	intlog map[string][]string
}

func NewPointData(name string, pts []*influxdb.Point) (*pointData, error) {
	if name == "" {
		return nil, errors.New("name doesnot empty")
	}

	var p = pointData{
		name:   name,
		data:   []map[string]interface{}{},
		intlog: map[string][]string{},
	}

	for _, pt := range pts {
		f, err := pt.Fields()
		if err != nil {
			return nil, err
		}

		p.data = append(p.data, map[string]interface{}{
			"name":   pt.Name(),
			"tags":   pt.Tags(),
			"fields": f,
			"time":   pt.UnixNano(),
		})
		p.intlog[pt.Name()] = integerLog(f)
	}

	return &p, nil
}

func (p *pointData) Name() string {
	return p.name
}

func (p *pointData) DataToLua() interface{} {
	return p.data
}

func (*pointData) CallbackFnName() string {
	return "handle"
}

func (*pointData) CallbackTypeName() string {
	return "points"
}

func (p *pointData) Handle(value string, err error) {
	if err != nil {
		fmt.Printf("receive error: %v\n", err)
		return
	}

	fmt.Printf("jsonStr: %s\n\n", value)

	type pd struct {
		Name   string                 `json:"name"`
		Tags   map[string]string      `json:"tags,omitempty"`
		Fields map[string]interface{} `json:"fields"`
		Time   int64                  `json:"time,omitempty"`
	}

	x := []pd{}
	json.Unmarshal([]byte(value), &x)

	pts := []*influxdb.Point{}
	for _, m := range x {
		recoveType(p.intlog[m.Name], m.Fields)

		pt, _ := influxdb.NewPoint(m.Name, m.Tags, m.Fields, time.Unix(0, m.Time))
		pts = append(pts, pt)
	}

	for _, pt := range pts {
		fmt.Println(pt.String())
	}
}

func integerLog(m map[string]interface{}) []string {
	var list []string
	for k, v := range m {
		switch v.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			list = append(list, k)
		}
	}
	return list
}

func recoveType(intlog []string, m map[string]interface{}) {
	for _, k := range intlog {
		switch m[k].(type) {
		case float64:
			m[k] = int64(m[k].(float64))
		}
	}
}

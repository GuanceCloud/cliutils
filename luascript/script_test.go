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
		for k, v in pairs(pt) do
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
	err = AddLuaLines("ptdata", []string{luaCode})
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
	name    string
	data    []map[string]interface{}
	typelog map[string]fieldType
}

func NewPointData(name string, pts []*influxdb.Point) (*pointData, error) {
	if name == "" {
		return nil, errors.New("name doesnot empty")
	}

	var p = pointData{name: name, data: []map[string]interface{}{}}

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
	}
	p.typelog = logType(pts)

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
		pt, _ := influxdb.NewPoint(m.Name, m.Tags, m.Fields, time.Unix(0, m.Time))
		pts = append(pts, pt)
	}

	pts, err = typeRecove(pts, p.typelog)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, pt := range pts {
		fmt.Println(pt.String())
	}
}

// log only `int' fields
type fieldType []string

func logType(pts []*influxdb.Point) map[string]fieldType {
	fts := map[string]fieldType{}
	for _, p := range pts {
		fts[p.Name()] = filterIntFields(p)
	}
	return fts
}

func filterIntFields(pt *influxdb.Point) fieldType {
	ft := fieldType{}
	fs, err := pt.Fields()
	if err != nil {
		return nil
	}

	for k, v := range fs {
		switch v.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			ft = append(ft, k)
		}
	}
	return ft
}

func typeRecove(pts []*influxdb.Point, typelog map[string]fieldType) ([]*influxdb.Point, error) {
	var points []*influxdb.Point
	for _, pt := range pts {
		newpt, err := recoverIntFields(pt, typelog[pt.Name()])
		if err != nil {
			return nil, err
		}
		points = append(points, newpt)
	}
	return points, nil
}

func recoverIntFields(p *influxdb.Point, ft fieldType) (*influxdb.Point, error) {
	if len(ft) == 0 { // FIXME: need new point based on @p?
		return p, nil
	}

	fs, err := p.Fields()
	if err != nil {
		return nil, err
	}

	pn := p.Name()

	n := 0

	// NOTE: Lua do not distinguish int/float, all Golang got is float.
	// if your really need int to be float, disable type-safe in configure.
	// Loop all original int fields, they must be float now, convert to int anyway.
	// We do not check other types of fields, the Lua developer SHOULD becarefull
	// to treat them type-safe when updating exists field values, or influxdb
	// may refuse to accept the point handled by Lua.
	for _, k := range ft {
		if fs[k] == nil {
			// l.Debugf("ignore missing filed %s.%s", pn, k)
			continue
		}
		switch fs[k].(type) {
		case float32:
			fs[k] = int64(fs[k].(float32))
			n++
		case float64:
			fs[k] = int64(fs[k].(float64))
			n++
		default:
			// l.Warnf("overwrite int field(%s.%s) with conflict type: int > %v, point: %s, ft: %v",pn, k, fs[k], p.String(), ft)
		}
	}

	if n == 0 { // no field updated
		return p, nil
	} else {
		// l.Debugf("%d points type recovered", n)
		pt, err := influxdb.NewPoint(pn, p.Tags(), fs, p.Time())
		if err != nil {
			// l.Error(err)
			return nil, err
		}
		return pt, nil
	}
}

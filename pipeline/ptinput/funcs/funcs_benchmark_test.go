// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package funcs

import (
	"strconv"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/GuanceCloud/platypus/pkg/ast"
)

/*
//  时间解析补充函数
func BenchmarkParseDatePattern(b *testing.B) {
	for n := 0; n < b.N; n++ {
		if _, err := parseDatePattern("Tue May 18 06:25:05.176170 2021"); err != nil {
			b.Error(err)
		}
	}
}

// dataparse库
func BenchmarkDateparseParseIn(b *testing.B) {
	for n := 0; n < b.N; n++ {
		if _, err := dateparse.ParseIn("2017-12-29T12:33:33.095243Z", nil); err != nil {
			b.Error(err)
		}
	}
}

// dataparse库 指定时区
func BenchmarkDateparseParseInTZ(b *testing.B) {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	for n := 0; n < b.N; n++ {
		if _, err := dateparse.ParseIn("2017-12-29T12:33:33.095243Z", tz); err != nil {
			b.Error(err)
		}
	}
}

// default_time， pipeline 时间解析函数
func BenchmarkTimeDefault(b *testing.B) {
	for n := 0; n < b.N; n++ {
		p := &Pipeline{
			Output: map[string]interface{}{
				"time": "2017-12-29T12:33:33.095243Z",
			},
			ast: &parser.Ast{
				Functions: []*parser.FuncExpr{
					{
						Name: "default_time",
						Param: []parser.Node{
							&parser.Identifier{Name: "time"},
							&parser.StringLiteral{Val: ""},
						},
					},
				},
			},
		}
		if _, err := DefaultTime(p, p.ast.Functions[0]); err != nil {
			b.Error(err)
		}
	}
}

// // default_time， pipeline 时间解析函数, 设置时区
func BenchmarkTimeDefaultTZ(b *testing.B) {
	for n := 0; n < b.N; n++ {
		p := &Pipeline{
			Output: map[string]interface{}{
				"time": "2017-12-29T12:33:33.095243Z",
			},
			ast: &parser.Ast{
				Functions: []*parser.FuncExpr{
					{
						Name: "default_time",
						Param: []parser.Node{
							&parser.Identifier{Name: "time"},
							&parser.StringLiteral{Val: "Asia/Shanghai"},
						},
					},
				},
			},
		}
		if _, err := DefaultTime(p, p.ast.Functions[0]); err != nil {
			b.Error(err)
		}
	}
}

// add_pattern 函数， pipeline 添加模式
func BenchmarkAddPattern(b *testing.B) {
	for n := 0; n < b.N; n++ {
		p := &Pipeline{
			Output: map[string]interface{}{
				"time": "2017-12-29T12:33:33.095243Z",
			},
			ast: &parser.Ast{
				Functions: []*parser.FuncExpr{
					{
						Name: "add_pattern",
						Param: []parser.Node{
							&parser.StringLiteral{Val: "time1"},
							&parser.StringLiteral{Val: "[\\w:\\.\\+-]+?"},
						},
					},
				},
			},
		}
		if _, err := AddPattern(p, p.ast.Functions[0]); err != nil {
			b.Error(err)
		}
	}
}

// default_time_with_fmt， pipeline 根据指定的时间 fmt 解析时间
func BenchmarkTimeDefaultWithTfmt(b *testing.B) {
	for n := 0; n < b.N; n++ {
		p := &Pipeline{
			Output: map[string]interface{}{
				"time": "2017-12-29T12:33:33.095243Z", // "2017-12-29T12:33:33.095243Z+0800"
			},
			ast: &parser.Ast{
				Functions: []*parser.FuncExpr{
					{
						Name: "default_time_with_fmt",
						Param: []parser.Node{
							&parser.Identifier{Name: "time"},
							&parser.StringLiteral{Val: "2006-01-02T15:04:05.000000Z"}, // 2006-01-02T15:04:05.000000Z-0700
							&parser.StringLiteral{Val: ""},                            // "Asia/Shanghai"
						},
					},
				},
			},
		}
		if _, err := DefaultTimeWithFmt(p, p.ast.Functions[0]); err != nil {
			b.Error(err)
		}
	}
}
*/

// default_time
func BenchmarkParseLog(b *testing.B) {
	script := `
	add_pattern("date1", "[\\w:\\.\\+-]+?")
	add_pattern("date2", "[\\w:\\.\\+-]+?")
	add_pattern("date3", "[\\w:\\.\\+-]+?")
	add_pattern("date4", "[\\w:\\.\\+-]+?")
	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{INT:thread_id}\\s+%{WORD:operation}\\s+%{GREEDYDATA:raw_query}")
	cast(thread_id, "int")
	default_time(time)
		`
	runner, err := NewTestingRunner(script)
	if err != nil {
		b.Error(err)
	}
	data := `2017-12-29T12:33:33.095243Z         2 Query     SELECT TABLE_SCHEMA, TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE CREATE_OPTIONS LIKE '%partitioned%'`

	for n := 0; n < b.N; n++ {
		pt := ptinput.NewPlPt(
			point.Logging, "test", nil, map[string]any{"message": data}, time.Now())
		errR := runScript(runner, pt)

		if errR != nil {
			b.Fatal(errR)
		}
	}
}

func BenchmarkParseLog_tz(b *testing.B) {
	script := `
	add_pattern("date1", "(\\d+/\\w+/[\\d:]+ [\\d+-]+)")
	add_pattern("date2", "[\\w:\\.\\+-]+?")
	add_pattern("date3", "[\\w:\\.\\+-]+?")
	add_pattern("date4", "[\\w:\\.\\+-]+?")
	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{IP:ip}\\s+%{INT:thread_id}\\s+")

	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{IP:ip}\\s+%{INT:thread_id}\\s+")
	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{IP:ip}\\s+%{INT:thread_id}\\s+")
	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{IP:ip}\\s+%{INT:thread_id}\\s+")


	cast(thread_id, "int")
	default_time(time, "Asia/Shanghai")
		`
	runner, err := NewTestingRunner(script)
	if err != nil {
		b.Error(err)
	}
	data := `2017-12-29T12:33:33.095243Z     1.1.1.1    2 `

	for n := 0; n < b.N; n++ {
		pt := ptinput.NewPlPt(
			point.Logging, "test", nil, map[string]any{"message": data}, time.Now())
		errR := runScript(runner, pt)

		if errR != nil {
			b.Fatal(errR)
		}
	}
}

func BenchmarkGrok(b *testing.B) {
	script := `
#grok(_, "%{IPV6:client_ip}")
#grok(_, "%{IPV6:client_ip} %{NOTSPACE:http_ident} %{NOTSPACE:http_auth} \\[%{HTTPDATE:time}\\] \"%{DATA:http_method} %{URIPATHPARAM:http_url} HTTP/%{NUMBER:http_version}\" %{INT:status_code} %{INT:bytes}")
grok(_, "%{IPORHOST:client_ip} %{NOTSPACE:http_ident} %{NOTSPACE:http_auth} \\[%{HTTPDATE:time}\\] \"%{DATA:http_method} %{GREEDYDATA:http_url} HTTP/%{NUMBER:http_version}\" %{INT:status_code} %{INT:bytes}")
`
	runner, err := NewTestingRunner(script)
	if err != nil {
		b.Error(err)
	}
	data := `127.0.0.1 - - [21/Jul/2021:14:14:38 +0800] "GET /?1 HTTP/1.1" 200 2178 "-" "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.164 Safari/537.36"`
	// data := `fe80:d::127.0.0.1 - - [21/Jul/2021:14:14:38 +0800] "GET /?1 HTTP/1.1" 200 2178 "-" "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.164 Safari/537.36"`

	pt := ptinput.NewPlPt(
		point.Logging, "test", nil, map[string]any{"message": data}, time.Now())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		errR := runScript(runner, pt)

		if errR != nil {
			b.Fatal(errR)
		}
	}
}

const lp = `gin app="deployment-forethought-kodo-kodo",client_ip="172.1***03",cluster_name_k8s="k8s-daily",container_id="dcbacc667c1534127d4f4c531fc26f613f4e6f822e646dee4e4bdbc5e87920c4",container_name="kodo",deployment="kodo",filepath="/rootfs/var/log/pods/forethought-kodo_kodo-7dc8b5c448-rmcpb_bd5159c7-df57-4346-987d-fc6883aeabea/kodo/0.log",guance_site="daily",host="cluster_a_cn-hangzhou.172.1***.102",host_ip="172.1***.102",image="registry.****.com/ko**:testing-202*****",log_read_lines=289892,message="[GIN] 2024/11/15 - 10:56:07 | 403 | 759.859µs |  172.16.200.203 | POST    \"/v1/write/metric?token=****************842cda605c6cb87e3a7b8\"",message_length=137,namespace="forethought-kodo",pod-template-hash="7dc8b5c448",pod_ip="10.113.0.204",pod_name="kodo-7dc8b5c448-rmcpb",real_host="hz-dataflux-daily-002",region="cn-hangzhou",service="kodo",status="warning",time_ns=1731639367526632400,time_us=1731639367526632,timestamp="2024/11/15 - 10:56:07",zone_id="cn-hangzhou-j" 1731639367526000000`

func BenchmarkParseLogNginx(b *testing.B) {
	script := `
add_pattern("date2", "%{YEAR}[./]%{MONTHNUM}[./]%{MONTHDAY} %{TIME}")

# access log
grok(_, "%{NOTSPACE:client_ip} %{NOTSPACE:http_ident} %{NOTSPACE:http_auth} \\[%{HTTPDATE:time}\\] \"%{DATA:http_method} %{GREEDYDATA:http_url} HTTP/%{NUMBER:http_version}\" %{INT:status_code} %{INT:bytes}")

# access log
add_pattern("access_common", "%{NOTSPACE:client_ip} %{NOTSPACE:http_ident} %{NOTSPACE:http_auth} \\[%{HTTPDATE:time}\\] \"%{DATA:http_method} %{GREEDYDATA:http_url} HTTP/%{NUMBER:http_version}\" %{INT:status_code} %{INT:bytes}")
grok(_, '%{access_common} "%{NOTSPACE:referrer}" "%{GREEDYDATA:agent}"')
user_agent(agent)

# error log
grok(_, "%{date2:time} \\[%{LOGLEVEL:status}\\] %{GREEDYDATA:msg}, client: %{NOTSPACE:client_ip}, server: %{NOTSPACE:server}, request: \"%{DATA:http_method} %{GREEDYDATA:http_url} HTTP/%{NUMBER:http_version}\", (upstream: \"%{GREEDYDATA:upstream}\", )?host: \"%{NOTSPACE:ip_or_host}\"")
grok(_, "%{date2:time} \\[%{LOGLEVEL:status}\\] %{GREEDYDATA:msg}, client: %{NOTSPACE:client_ip}, server: %{NOTSPACE:server}, request: \"%{GREEDYDATA:http_method} %{GREEDYDATA:http_url} HTTP/%{NUMBER:http_version}\", host: \"%{NOTSPACE:ip_or_host}\"")
grok(_,"%{date2:time} \\[%{LOGLEVEL:status}\\] %{GREEDYDATA:msg}")

group_in(status, ["warn", "notice"], "warning")
group_in(status, ["error", "crit", "alert", "emerg"], "error")

cast(status_code, "int")
cast(bytes, "int")

group_between(status_code, [200,299], "OK", status)
group_between(status_code, [300,399], "notice", status)
group_between(status_code, [400,499], "warning", status)
group_between(status_code, [500,599], "error", status)

`
	runner, err := NewTestingRunner(script)
	if err != nil {
		b.Error(err)
	}

	b.ResetTimer()
	dec := point.GetDecoder()
	v, _ := dec.Decode([]byte(lp))
	pt := v[0]
	data := `127.0.0.1 - - [21/Jul/2021:14:14:38 +0800] "GET /?1 HTTP/1.1" 200 2178 "-" "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.164 Safari/537.36"`
	b.ResetTimer()
	pt.MustAdd("message", data)

	b.Run("pl-old", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			plpt := ptinput.WrapPoint(
				point.Logging, pt)
			errR := runScript(runner, plpt)
			npt := plpt.Point()
			_ = npt
			if errR != nil {
				b.Fatal(errR)
			}
		}
	})

	b.Run("pl-new", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			plpt := ptinput.PtWrap(
				point.Logging, pt)
			errR := runScript(runner, plpt)
			npt := plpt.Point()
			_ = npt
			if errR != nil {
				b.Fatal(errR)
			}
		}
	})
}

func BenchmarkPtWrap(b *testing.B) {
	messages := `127.0.0.1 - - [21/Jul/2021:14:14:38 +0800] "GET /?1 HTTP/1.1" 200 2178 "-" "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.164 Safari/537.36"`

	kvs := point.KVs{}
	kvs = kvs.Add("1", 1, false, true)
	kvs = kvs.Add("2", 2, false, true)
	kvs = kvs.Add("message", messages, false, true)
	kvs = kvs.Add("f", 3.14, false, true)
	kvs = kvs.Add("4", 4, false, true)
	kvs = kvs.Add("5", 5, false, true)
	kvs = kvs.Add("s", "aaaaaaaaaaa", false, true)
	pt := point.NewPointV2("abc", kvs, point.WithTime(time.Now()))

	fn := func(pp ptinput.PlInputPt) {
		for i := 0; i < 50; i++ {
			pp.Set(strconv.Itoa(i), i, ast.Int)
		}
		pp.Set("message", messages, ast.String)
		for i := 50; i < 70; i++ {
			v := strconv.Itoa(i) + "_xxxxx"
			pp.Set(v, v, ast.String)
		}
		for i := 0; i < 15; i++ {
			_, _, _ = pp.Get("message")
		}

		for i := 40; i < 50; i++ {
			pp.Delete(strconv.Itoa(i))
		}
		for i := 50; i < 55; i++ {
			pp.Delete(strconv.Itoa(i))
		}
		for i := 55; i < 60; i++ {
			v := strconv.Itoa(i) + "_xxxxx"
			pp.Delete(v)
		}
	}

	b.ResetTimer()
	b.Run("old", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			pp := ptinput.WrapPoint(
				point.Logging, pt)
			fn(pp)
			a := pp.Point()
			_ = a
			_ = pp
		}
	})

	b.Run("new", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			pp := ptinput.PtWrap(
				point.Logging, pt)
			fn(pp)
			a := pp.Point()
			_ = a
			_ = pp
		}
	})

	// b.Run("new-pool-pt", func(b *testing.B) {
	// 	point.SetPointPool(point.NewReservedCapPointPool(256))
	// 	for n := 0; n < b.N; n++ {
	// 		pp := ptinput.PtWrap(
	// 			point.Logging, pt)
	// 		fn(pp)
	// 		// pt := pp.Point()
	// 		// _ = pt
	// 		point.GetPointPool().Put(pt)
	// 		_ = pp
	// 	}
	// })
}

// default_time_with_fmt
func BenchmarkParseLogWithTfmt(b *testing.B) {
	script := `
	add_pattern("date1", "[\\w:\\.\\+-]+?")
	add_pattern("date2", "[\\w:\\.\\+-]+?")
	add_pattern("date3", "[\\w:\\.\\+-]+?")
	add_pattern("date4", "[\\w:\\.\\+-]+?");	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{INT:thread_id}\\s+%{WORD:operation}\\s+%{GREEDYDATA:raw_query}")
	cast(thread_id, "int")
	default_time_with_fmt(time, "2006-01-02T15:04:05.000000Z")
	`
	// "2006-01-02T15:04:05.000000Z"
	runner, err := NewTestingRunner(script)
	if err != nil {
		b.Error(err)
	}
	data := `2021-07-20T12:33:33.095243Z         2 Query     SELECT TABLE_SCHEMA, TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE CREATE_OPTIONS LIKE '%partitioned%'`

	for n := 0; n < b.N; n++ {
		pt := ptinput.NewPlPt(
			point.Logging, "test", nil, map[string]any{"message": data}, time.Now())
		errR := runScript(runner, pt)

		if errR != nil {
			b.Fatal(errR)
		}
	}
}

// default_time_with_fmt， timezone
func BenchmarkParseLogWithTfmt_tz(b *testing.B) {
	script := `
	add_pattern("date1", "[\\w:\\.\\+-]+?")
	add_pattern("date2", "[\\w:\\.\\+-]+?")
	add_pattern("date3", "[\\w:\\.\\+-]+?")
	add_pattern("date4", "[\\w:\\.\\+-]+?")
	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{INT:thread_id}\\s+%{WORD:operation}\\s+%{GREEDYDATA:raw_query}")
	cast(thread_id, "int")
	default_time_with_fmt(time, "2006-01-02T15:04:05.000000Z", "Asia/Shanghai")
	`
	// "2006-01-02T15:04:05.000000Z"
	runner, err := NewTestingRunner(script)
	if err != nil {
		b.Error(err)
	}
	data := `2021-07-20T12:33:33.095243Z         2 Query     SELECT TABLE_SCHEMA, TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE CREATE_OPTIONS LIKE '%partitioned%'`

	for n := 0; n < b.N; n++ {
		pt := ptinput.NewPlPt(
			point.Logging, "test", nil, map[string]any{"message": data}, time.Now())
		errR := runScript(runner, pt)

		if errR != nil {
			b.Fatal(errR)
		}
	}
}

func BenchmarkParseLogWithTfmt_NoAddPattern(b *testing.B) {
	script := `
	grok(_, "%{TIMESTAMP_ISO8601:time}\\s+%{INT:thread_id}\\s+%{WORD:operation}\\s+%{GREEDYDATA:raw_query}")
	cast(thread_id, "int")
	default_time_with_fmt(time, "2006-01-02T15:04:05.000000Z", "Asia/Shanghai")
	`
	// "2006-01-02T15:04:05.000000Z"
	runner, err := NewTestingRunner(script)
	if err != nil {
		b.Error(err)
	}
	data := `2021-07-20T12:33:33.095243Z         2 Query     SELECT TABLE_SCHEMA, TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE CREATE_OPTIONS LIKE '%partitioned%'`

	for n := 0; n < b.N; n++ {
		pt := ptinput.NewPlPt(
			point.Logging, "test", nil, map[string]any{"message": data}, time.Now())
		errR := runScript(runner, pt)

		if errR != nil {
			b.Fatal(errR)
		}
	}
}

package point

import (
	"fmt"
	sync "sync"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var sampleLogs = []string{
	`2022-10-27T16:12:54.876+0800	DEBUG	ddtrace	trace/filters.go:235	keep tid: 971624677789410817 service: compiled-in-example resource: file-not-exists according to PRIORITY_AUTO_KEEP and sampling ratio: 100%                                                                `,
	`2022-10-27T16:12:54.876+0800	DEBUG	ddtrace	trace/filters.go:235	keep tid: 564726768482716036 service: compiled-in-example resource: ./demo according to PRIORITY_AUTO_KEEP and sampling ratio: 100%`,
	`2022-10-27T16:12:54.876+0800	DEBUG	ddtrace	trace/filters.go:102	keep tid: 971624677789410817 service: compiled-in-example resource: file-not-exists according to PRIORITY_AUTO_KEEP.`,
	`2022-10-27T16:12:54.876+0800	DEBUG	ddtrace	trace/filters.go:102	keep tid: 564726768482716036 service: compiled-in-example resource: ./demo according to PRIORITY_AUTO_KEEP.`,
	`2022-10-27T16:12:54.876+0800	DEBUG	ddtrace	trace/aftergather.go:121	### send 2 points cost 0ms with error: <nil>`,
	`2022-10-27T16:12:54.875+0800	DEBUG	ddtrace	ddtrace/ddtrace_http.go:34	### received tracing data from path: /v0.4/traces`,
	`2022-10-27T16:12:54.281+0800	DEBUG	filter	filter/filter.go:158	filter condition body: {"dataways":null,"filters":{"logging":["{ source =  'datakit'  and ( host in [ 'ubt-dev-01' ,  'tanb-ubt-dev-test' ] )}"]},"pull_interval":10000000000,"remote_pipelines":null}`,
	`2022-10-27T16:12:54.184+0800	DEBUG	io	io/io.go:97	get iodata(1 points) from /v1/write/metric|swap`,
	`2022-10-27T16:12:54.184+0800	DEBUG	filter	filter/filter.go:408	update metrics...`,
	`2022-10-27T16:12:54.184+0800	DEBUG	filter	filter/filter.go:401	try pull remote filters...`,
	`2022-10-27T16:12:54.184+0800	DEBUG	filter	filter/filter.go:262	/v1/write/metric/pts: 1, after: 1`,
	`2022-10-27T16:12:54.184+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:54.184+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=GET, string=url, *url.URL=https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true)`,
	`2022-10-27T16:12:54.183+0800	DEBUG	io	io/feed.go:91	io feed swap|/v1/write/metric`,
	`2022-10-27T16:12:54.183+0800	DEBUG	filter	filter/filter.go:235	no condition filter for metric`,
	`2022-10-27T16:12:53.688+0800	DEBUG	filter	filter/filter.go:158	filter condition body: {"dataways":null,"filters":{"logging":["{ source =  'datakit'  and ( host in [ 'ubt-dev-01' ,  'tanb-ubt-dev-test' ] )}"]},"pull_interval":10000000000,"remote_pipelines":null}`,
	`2022-10-27T16:12:53.622+0800	DEBUG	io	io/io.go:97	get iodata(2 points) from /v1/write/tracing|ddtrace`,
	`2022-10-27T16:12:53.622+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:49.573+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush dynamicDatawayCategory(0 pts), last flush 9.999510666s ago...`,
	`2022-10-27T16:12:49.462+0800	DEBUG	filter	filter/filter.go:158	filter condition body: {"dataways":null,"filters":{"logging":["{ source =  'datakit'  and ( host in [ 'ubt-dev-01' ,  'tanb-ubt-dev-test' ] )}"]},"pull_interval":10000000000,"remote_pipelines":null}`,
	`2022-10-27T16:12:49.389+0800	DEBUG	filter	filter/filter.go:401	try pull remote filters...`,
	`2022-10-27T16:12:49.389+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:49.389+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=GET, string=url, *url.URL=https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true)`,
	`2022-10-27T16:12:49.388+0800	DEBUG	filter	filter/filter.go:408	update metrics...`,
	`2022-10-27T16:12:49.388+0800	DEBUG	filter	filter/filter.go:158	filter condition body: {"dataways":null,"filters":{"logging":["{ source =  'datakit'  and ( host in [ 'ubt-dev-01' ,  'tanb-ubt-dev-test' ] )}"]},"pull_interval":10000000000,"remote_pipelines":null}`,
	`2022-10-27T16:12:49.386+0800	DEBUG	io	io/io.go:97	get iodata(4 points) from /v1/write/tracing|ddtrace`,
	`2022-10-27T16:12:48.636+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:48.636+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=GET, string=url, *url.URL=https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true)`,
	`2022-10-27T16:12:48.444+0800	DEBUG	filter	filter/filter.go:158	filter condition body: {"dataways":null,"filters":{"logging":["{ source =  'datakit'  and ( host in [ 'ubt-dev-01' ,  'tanb-ubt-dev-test' ] )}"]},"pull_interval":10000000000,"remote_pipelines":null}`,
	`2022-10-27T16:12:48.400+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:48.400+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=GET, string=url, *url.URL=https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true)`,
	`2022-10-27T16:12:46.815+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:46.815+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=GET, string=url, *url.URL=https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true)`,
	`2022-10-27T16:12:46.726+0800	DEBUG	filter	filter/filter.go:158	filter condition body: {"dataways":null,"filters":{"logging":["{ source =  'datakit'  and ( host in [ 'ubt-dev-01' ,  'tanb-ubt-dev-test' ] )}"]},"pull_interval":10000000000,"remote_pipelines":null}`,
	`2022-10-27T16:12:46.703+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=POST, string=url, *url.URL=https://openway.guance.com/v1/write/tracing?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588)`,
	`2022-10-27T16:12:46.700+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/write/tracing?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:46.699+0800	DEBUG	sender	sender/sender.go:47	sending /v1/write/object(1 pts)...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	sender	sender/sender.go:47	sending /v1/write/metric(1 pts)...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:270	wal try flush failed data on /v1/write/security`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:270	wal try flush failed data on /v1/write/rum`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:270	wal try flush failed data on /v1/write/network`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:270	wal try flush failed data on /v1/write/metric`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:270	wal try flush failed data on /v1/write/logging`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/security(0 pts), last flush 10.000030625s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/rum(0 pts), last flush 9.999880583s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/rum(0 pts), last flush 9.999536583s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/rum(0 pts), last flush 9.999386542s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/rum(0 pts), last flush 9.999338708s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/rum(0 pts), last flush 9.998867333s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/rum(0 pts), last flush 9.998208209s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/object(1 pts), last flush 9.997395583s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/network(0 pts), last flush 9.99991425s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/network(0 pts), last flush 9.999568875s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/network(0 pts), last flush 9.998325375s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/network(0 pts), last flush 9.998172s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/network(0 pts), last flush 9.997431792s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(1 pts), last flush 9.999472083s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(0 pts), last flush 9.999964541s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(0 pts), last flush 9.999953542s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(0 pts), last flush 9.999944333s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(0 pts), last flush 9.999897792s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(0 pts), last flush 9.999869417s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(0 pts), last flush 9.999858791s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/metric(0 pts), last flush 9.99767025s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/logging(0 pts), last flush 9.999887125s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/logging(0 pts), last flush 9.998371916s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/logging(0 pts), last flush 9.997611625s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/logging(0 pts), last flush 9.997412708s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/logging(0 pts), last flush 10.002298833s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/logging(0 pts), last flush 10.000082958s ago...`,
	`2022-10-27T16:12:46.699+0800	DEBUG	io	io/io.go:265	on tick(10s) to flush /v1/write/logging(0 pts), last flush 10.000006916s ago...`,
	`2022-10-27T16:12:46.306+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:46.306+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=GET, string=url, *url.URL=https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true)`,
	`2022-10-27T16:12:46.305+0800	DEBUG	ddtrace	trace/filters.go:235	keep tid: 2790747027482021869 service: compiled-in-example resource: ./demo according to PRIORITY_AUTO_KEEP and sampling ratio: 100%`,
	`2022-10-27T16:12:46.305+0800	DEBUG	ddtrace	trace/filters.go:235	keep tid: 1965248471827589152 service: compiled-in-example resource: file-not-exists according to PRIORITY_AUTO_KEEP and sampling ratio: 100%`,
	`2022-10-27T16:12:46.305+0800	DEBUG	ddtrace	trace/filters.go:102	keep tid: 2790747027482021869 service: compiled-in-example resource: ./demo according to PRIORITY_AUTO_KEEP.`,
	`2022-10-27T16:12:46.305+0800	DEBUG	ddtrace	trace/filters.go:102	keep tid: 1965248471827589152 service: compiled-in-example resource: file-not-exists according to PRIORITY_AUTO_KEEP.`,
	`2022-10-27T16:12:45.481+0800	DEBUG	disk	disk/utils.go:62	disk---fstype:nullfs ,device:/Applications/网易有道词典.app ,mountpoint:/private/var/folders/71/4pnfjgwn0x3fcy4r3ddxw1fm0000gn/T/AppTranslocation/1A552256-4134-4CAA-A4FF-7D2DEF11A6AC`,
	`2022-10-27T16:12:45.481+0800	DEBUG	disk	disk/utils.go:62	disk---fstype:nullfs ,device:/Applications/oss-browser.app ,mountpoint:/private/var/folders/71/4pnfjgwn0x3fcy4r3ddxw1fm0000gn/T/AppTranslocation/97346A30-EA8C-4AC8-991D-3AD64E2479E1`,
	`2022-10-27T16:12:45.481+0800	DEBUG	disk	disk/utils.go:62	disk---fstype:nullfs ,device:/Applications/Sublime Text.app ,mountpoint:/private/var/folders/71/4pnfjgwn0x3fcy4r3ddxw1fm0000gn/T/AppTranslocation/0EE2FB5D-6535-47AB-938B-DCB79CE11CE6`,
	`2022-10-27T16:12:45.481+0800	DEBUG	disk	disk/utils.go:62	disk---fstype:nullfs ,device:/Applications/Microsoft Remote Desktop.app ,mountpoint:/private/var/folders/71/4pnfjgwn0x3fcy4r3ddxw1fm0000gn/T/AppTranslocation/DD10B11F-2D45-4DFD-B1CB-EF0F2B1FB2F7`,
	`2022-10-27T16:12:42.051+0800	DEBUG	ddtrace	trace/filters.go:235	keep tid: 5484031498000114328 service: compiled-in-example resource: ./demo according to PRIORITY_AUTO_KEEP and sampling ratio: 100%`,
	`2022-10-27T16:12:42.051+0800	DEBUG	ddtrace	trace/filters.go:235	keep tid: 1409415361793528756 service: compiled-in-example resource: file-not-exists according to PRIORITY_AUTO_KEEP and sampling ratio: 100%`,
	`2022-10-27T16:12:42.051+0800	DEBUG	ddtrace	trace/filters.go:102	keep tid: 1409415361793528756 service: compiled-in-example resource: file-not-exists according to PRIORITY_AUTO_KEEP.`,
	`2022-10-27T16:12:42.051+0800	DEBUG	ddtrace	trace/aftergather.go:121	### send 2 points cost 0ms with error: <nil>`,
	`2022-10-27T16:12:42.051+0800	DEBUG	dataway	dataway/send.go:219	send request https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true, proxy: , dwcli: 0x1400049e000, timeout: 30s(30s)`,
	`2022-10-27T16:12:42.051+0800	DEBUG	dataway	dataway/cli.go:27	performing request%!(EXTRA string=method, string=GET, string=url, *url.URL=https://openway.guance.com/v1/datakit/pull?token=tkn_2af4b19d7f5a489fa81f0fff7e63b588&filters=true)`,
	`2022-10-27T16:12:42.050+0800	DEBUG	ddtrace	trace/filters.go:102	keep tid: 5484031498000114328 service: compiled-in-example resource: ./demo according to PRIORITY_AUTO_KEEP.`,
	`2022-10-27T16:12:42.050+0800	DEBUG	ddtrace	ddtrace/ddtrace_http.go:34	### received tracing data from path: /v0.4/traces`,
}

func BenchmarkPoolV0(b *T.B) {
	now := time.Now()

	b.Run("without-pool", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			var kvs KVs

			kvs = kvs.Add("f0", 123, false, true)
			kvs = kvs.Add("f1", 3.14, false, true)
			kvs = kvs.Add("f2", "hello", false, true)
			kvs = kvs.Add("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text"), false, true)
			kvs = kvs.Add("f4", false, false, false)
			kvs = kvs.Add("f5", -123, false, false)

			NewPointV2("m1", kvs, WithTime(now), WithPrecheck(false))
		}
	})
}

func BenchmarkPoolV1(b *T.B) {
	now := time.Now()
	pp := NewPointPoolLevel1()

	b.Run("v1-pool", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			pt := pp.Get()

			pt.SetName("m1")
			pt.SetTime(now)
			pt.Add("f0", 123)
			pt.Add("f1", 3.14)
			pt.Add("f2", "hello")
			pt.Add("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text"))
			pt.Add("f4", false)
			pt.Add("f5", -123)

			pp.Put(pt)
		}
	})
}

func BenchmarkPoolV2(b *T.B) {
	now := time.Now()

	var ppp partialPointPool

	b.Run("v2-pool", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			pt := ppp.Get()

			pt.SetName("m1")
			pt.SetTime(now)

			pt.AddKVs(ppp.GetKV("f0", 123),
				ppp.GetKV("f1", 3.14),
				ppp.GetKV("f2", "hello"),
				ppp.GetKV("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text")),
				ppp.GetKV("f4", false),
				ppp.GetKV("f5", -123))

			ppp.Put(pt)
		}
	})
}

func BenchmarkPoolV3(b *T.B) {
	now := time.Now()

	b.Run("v3-pool", func(b *T.B) {
		fpp := NewPointPoolLevel3()

		defer func() {

			SetPointPool(nil)
		}()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pt := fpp.Get()

			pt.SetName("m1")
			pt.SetTime(now)

			pt.AddKVs(
				fpp.GetKV("f0", 123),
				fpp.GetKV("f1", 3.14),
				fpp.GetKV("f2", "hello"),
				fpp.GetKV("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text")),
				fpp.GetKV("f4", false),
				fpp.GetKV("f5", -123))

			fpp.Put(pt)
		}
	})

	b.Run("v3-new-point", func(b *T.B) {
		fpp := NewPointPoolLevel3()
		SetPointPool(fpp)
		defer func() {

			SetPointPool(nil)
		}()

		b.ResetTimer()
		var kvs KVs
		for i := 0; i < b.N; i++ {
			kvs = kvs.Add("f0", 123, false, false)
			kvs = kvs.Add("f1", 3.14, false, false)
			kvs = kvs.Add("f2", "hello", false, false)
			kvs = kvs.Add("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text"), false, false)
			kvs = kvs.Add("f4", false, false, false)
			kvs = kvs.Add("f5", -123, false, false)

			pt := NewPointV2("m1", kvs, WithPrecheck(false))
			fpp.Put(pt)
		}
	})
}

func TestPointPool(t *T.T) {
	t.Run("level-1", func(t *T.T) {
		pp := NewPointPoolLevel1()

		pt := pp.Get()
		pt.SetName("m1")
		pt.AddKVs(pp.GetKV("f0", 123),
			pp.GetKV("f1", 3.14),
			pp.GetKV("f2", "hello"),
			pp.GetKV("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text")),
			pp.GetKV("f4", false),
			pp.GetKV("f5", -123))
		pp.Put(pt)

		pt = pp.Get()
		assert.True(t, isEmptyPoint(pt))

		assert.True(t, cap(pt.pt.Fields) > 0) // reuse field array
	})

	t.Run("level3-put-kv", func(t *T.T) {
		pp := &fullPointPool{}
		SetPointPool(pp)
		defer ClearPointPool()

		// total add 100 * 4 Field
		for i := 0; i < 100; i++ {
			var kvs KVs
			kvs = kvs.Add(fmt.Sprintf("f%d", i), 123, false, false)
			kvs = kvs.Add(fmt.Sprintf("f%d", i+1), 123, false, false)
			kvs = kvs.Add(fmt.Sprintf("f%d", i+2), 123, false, false)
			kvs = kvs.Add(fmt.Sprintf("f%d", i+3), 123, false, false)

			for _, kv := range kvs {
				pp.PutKV(kv)
			}
		}

		assert.Equal(t, int64(396), pp.kvReused.Load())
		assert.Equal(t, int64(4), pp.kvCreated.Load())

		t.Logf("point pool: %s", pp)
	})

	t.Run("level-3-concurrent", func(t *T.T) {
		pp := &fullPointPool{}
		SetPointPool(pp)
		defer ClearPointPool()

		var wg sync.WaitGroup

		f := func() {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				var kvs KVs
				kvs = kvs.Add(fmt.Sprintf("f-%d", i*100), 123, false, false)
				kvs = kvs.Add(fmt.Sprintf("f-%d", i*100+1), 123, false, false)
				kvs = kvs.Add(fmt.Sprintf("f-%d", i*100+2), 123, false, false)
				kvs = kvs.Add(fmt.Sprintf("f-%d", i*100+3), 123, false, false)

				for _, kv := range kvs {
					pp.PutKV(kv)
				}
			}
		}

		n := 100
		wg.Add(n)
		for i := 0; i < n; i++ {
			go f()
		}

		wg.Wait()

		assert.Equal(t, int64(n*100*4), pp.kvReused.Load()+pp.kvCreated.Load())
		t.Logf("point pool: %s", pp)
	})
}

func TestReset(t *T.T) {
	t.Run("reset", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f1", 123, false, false)
		kvs = kvs.Add("f2", false, false, false)

		pt := NewPointV2("" /* go warnning */, kvs, WithTime(time.Now()))
		pt.Reset()

		assert.True(t, isEmptyPoint(pt))
	})
}

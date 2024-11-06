package point

import (
	"fmt"
	"math"
	sync "sync"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/metrics"
	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func BenchmarkNoReservedCapPool(b *T.B) {
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

func BenchmarkReserved1KCapPool(b *T.B) {
	pp := NewReservedCapPointPool(1000)
	SetPointPool(pp)
	defer func() {
		SetPointPool(nil)
	}()

	b.Cleanup(func() {
		metrics.Unregister(pp)
	})

	metrics.MustRegister(pp)

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
		pp.Put(pt)
	}
}

func BenchmarkReservedZeroCapPool(b *T.B) {
	pp := NewReservedCapPointPool(0)
	SetPointPool(pp)
	defer func() {
		SetPointPool(nil)
	}()

	b.Cleanup(func() {
		metrics.Unregister(pp)
	})

	metrics.MustRegister(pp)

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
		pp.Put(pt)
	}
}

func BenchmarkParallelNoPool(b *T.B) {
	now := time.Now()

	b.RunParallel(func(b *T.PB) {
		for b.Next() {
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

func BenchmarkParallelReserveCapPool(b *T.B) {
	b.RunParallel(func(b *T.PB) {
		pp := NewReservedCapPointPool(1000)
		SetPointPool(pp)
		defer func() {
			SetPointPool(nil)
		}()

		for b.Next() {
			var kvs KVs
			kvs = kvs.Add("f0", 123, false, false)
			kvs = kvs.Add("f1", 3.14, false, false)
			kvs = kvs.Add("f2", "hello", false, false)
			kvs = kvs.Add("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text"), false, false)
			kvs = kvs.Add("f4", false, false, false)
			kvs = kvs.Add("f5", -123, false, false)

			pt := NewPointV2("m1", kvs, WithPrecheck(false))
			pp.Put(pt)
		}
	})
}

func BenchmarkParallelNoReserveCapPool(b *T.B) {
	b.RunParallel(func(b *T.PB) {
		pp := NewReservedCapPointPool(0)
		SetPointPool(pp)
		defer func() {
			SetPointPool(nil)
		}()

		for b.Next() {
			var kvs KVs
			kvs = kvs.Add("f0", 123, false, false)
			kvs = kvs.Add("f1", 3.14, false, false)
			kvs = kvs.Add("f2", "hello", false, false)
			kvs = kvs.Add("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text"), false, false)
			kvs = kvs.Add("f4", false, false, false)
			kvs = kvs.Add("f5", -123, false, false)

			pt := NewPointV2("m1", kvs, WithPrecheck(false))
			pp.Put(pt)
		}
	})
}

func TestPointPool(t *T.T) {
	t.Run("reserve-cap-pool-put-kv", func(t *T.T) {
		pp := NewReservedCapPointPool(1000)
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

		cpp := pp.(*ReservedCapPointPool)

		assert.Equal(t, int64(4), cpp.poolGet())   // pool-get: new object from pool
		assert.Equal(t, int64(396), cpp.chanGet()) // chan-get: reuse-exist object from channel

		t.Logf("point pool: %s", cpp.String())
	})

	t.Run("point-pool-concurrent", func(t *T.T) {
		pp := NewReservedCapPointPool(1000)
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

		cpp := pp.(*ReservedCapPointPool)

		// all put-back locate in chan, not pool(here chan cap is 1000, too large for only 4 kvs each loop).
		assert.Equal(t, int64(n*100*4), cpp.chanPut())

		t.Logf("point pool: %s", cpp.String())
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

func BenchmarkStringKV(b *T.B) {
	var (
		pp = NewReservedCapPointPool(1000)

		shortString = cliutils.CreateRandomString(32)
		longString  = cliutils.CreateRandomString(1 << 10) // 1KB
		hugeString  = cliutils.CreateRandomString(1 << 20) // 1MB
		now         = time.Now()
	)

	SetPointPool(pp)
	defer ClearPointPool()

	b.Run("string-kv", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			var kvs KVs
			kvs = kvs.Add("short-f1", shortString, false, false)
			kvs = kvs.Add("long-f2", longString, false, false)
			kvs = kvs.Add("huge-f3", hugeString, false, false)

			kvs = kvs.Add("local-str-f1", "f1", false, false)
			kvs = kvs.Add("local-str-f2", "f2", false, false)
			kvs = kvs.Add("local-str-f3", "f3", false, false)

			pt := NewPointV2("m1",
				kvs,
				WithTime(now),
				WithStrField(true),
				WithPrecheck(false),
			)
			pp.Put(pt)
		}
	})

	b.Run("non-string-kv", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			var kvs KVs
			kvs = kvs.Add("f-i", 1024, false, false)
			kvs = kvs.Add("f-b", false, false, false)
			kvs = kvs.Add("f-f", 3.14, false, false)
			kvs = kvs.Add("f-u", uint(42), false, false)
			kvs = kvs.Add("f-max-int", int64(math.MaxInt64), false, false)
			kvs = kvs.Add("f-max-uint", uint64(math.MaxUint64), false, false)

			pt := NewPointV2("m1",
				kvs,
				WithTime(now),
				WithPrecheck(false),
			)
			pp.Put(pt)
		}
	})
}

func TestPointPoolMetrics(t *T.T) {
	t.Run("reserved-pool-metrics", func(t *T.T) {
		pp := NewReservedCapPointPool(100)
		SetPointPool(pp)
		defer ClearPointPool()

		t.Cleanup(func() {
			metrics.Unregister(pp)
		})

		metrics.MustRegister(pp)

		// total add 100 * 4 Field
		for i := 0; i < 100; i++ {
			func() {
				var kvs KVs
				kvs = kvs.Add(fmt.Sprintf("f%d", i), 123, false, false)
				kvs = kvs.Add(fmt.Sprintf("f%d", i+1), 123, false, false)
				kvs = kvs.Add(fmt.Sprintf("f%d", i+2), 123, false, false)
				kvs = kvs.Add(fmt.Sprintf("f%d", i+3), 123, false, false)

				pt := NewPointV2("some", kvs)
				pp.Put(pt)
			}()
		}

		_, err := metrics.Gather()
		assert.NoError(t, err)
	})
}

func TestReservedCapPointPool(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		pp := NewReservedCapPointPool(100)
		SetPointPool(pp)
		defer ClearPointPool()

		var kvs KVs
		kvs = kvs.Add(fmt.Sprintf("f%d", 0), 123, false, false)
		kvs = kvs.Add(fmt.Sprintf("f%d", +1), 123, false, false)
		kvs = kvs.Add(fmt.Sprintf("f%d", +2), 123, false, false)
		kvs = kvs.Add(fmt.Sprintf("f%d", +3), 123, false, false)

		pt := NewPointV2("some", kvs)
		t.Logf("pt: %s", pt.Pretty())

		pp.Put(pt)

		empty := pp.Get()
		t.Logf("empty pt: %s", empty.Pretty())
	})
}

func TestPoolEscape(t *T.T) {
	t.Run("escape", func(t *T.T) {
		// setup point pool
		pp := NewReservedCapPointPool(32)
		SetPointPool(pp)
		metrics.MustRegister(pp)

		enc := GetEncoder(WithEncEncoding(Protobuf))

		dec := GetDecoder(WithDecEncoding(Protobuf), WithDecEasyproto(false))

		r := NewRander()
		pts := r.Rand(100)

		t.Cleanup(func() {
			PutEncoder(enc)
			PutDecoder(dec)
			for _, pt := range pts {
				pp.Put(pt)
			}

			ClearPointPool()
			metrics.Unregister(pp)
		})

		enc.EncodeV2(pts)
		encBuf := make([]byte, 1<<20)
		for {
			if buf, ok := enc.Next(encBuf); ok {
				decPts, err := dec.Decode(buf)
				assert.NoError(t, err)

				for _, pt := range decPts {
					require.False(t, pt.HasFlag(Ppooled))
					pp.Put(pt)
				}
			} else {
				break
			}
		}

		mfs, err := metrics.Gather()
		assert.NoError(t, err)

		mf := metrics.GetMetric(mfs, "pointpool_escaped", 0)
		assert.Equal(t, 100.0, mf.GetCounter().GetValue()) // decoded 100 points(not easyproto) not from point pool
	})

	t.Run("no-escape", func(t *T.T) {
		// setup point pool
		pp := NewReservedCapPointPool(32)
		SetPointPool(pp)
		metrics.MustRegister(pp)

		enc := GetEncoder(WithEncEncoding(Protobuf))

		dec := GetDecoder(WithDecEncoding(Protobuf), WithDecEasyproto(true))

		r := NewRander()
		pts := r.Rand(100)

		t.Cleanup(func() {
			PutEncoder(enc)
			PutDecoder(dec)
			for _, pt := range pts {
				pp.Put(pt)
			}

			ClearPointPool()
			metrics.Unregister(pp)
		})

		enc.EncodeV2(pts)
		encBuf := make([]byte, 1<<20)
		for {
			if buf, ok := enc.Next(encBuf); ok {
				decPts, err := dec.Decode(buf)
				assert.NoError(t, err)

				for _, pt := range decPts {
					pp.Put(pt)
				}
			} else {
				break
			}
		}

		mfs, err := metrics.Gather()
		assert.NoError(t, err)

		mf := metrics.GetMetric(mfs, "pointpool_escaped", 0)
		assert.Equal(t, 0.0, mf.GetCounter().GetValue()) // decoded 100 points(not easyproto) not from point pool
	})
}

func TestPoolKVResuable(t *T.T) {
	type Foo struct {
		Measurement string

		TS int64

		// tags
		T1Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		T2Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		T3Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`

		T1 string
		T2 string
		T3 string

		SKey, S string `fake:"{regex:[a-zA-Z0-9]{128}}"`

		I8Key  string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		I16Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		I32Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		I64Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		I64    int64
		I8     int8
		I16    int16
		I32    int32

		U8Key  string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		U16Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		U32Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		U64Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`

		U8  uint8
		U16 uint16
		U32 uint32
		U64 uint64

		BKey   string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		DKey   string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		F64Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		F32Key string `fake:"{regex:[a-zA-Z0-9_]{64}}"`
		B      bool
		D      []byte
		F64    float64
		F32    float32
	}

	cases := []struct {
		name string
		pp   PointPool
	}{
		{
			name: "reserve-cap-pool",
			pp:   NewReservedCapPointPool(1024),
		},

		{
			name: "reserve-0-cap-pool", // regression to v3
			pp:   NewReservedCapPointPool(0),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			metrics.MustRegister(tc.pp)

			SetPointPool(tc.pp)
			t.Cleanup(func() {
				ClearPointPool()
				metrics.Unregister(tc.pp)
			})

			var f Foo
			maxPT := (1 << 20)
			for i := 0; i < maxPT; i++ {
				assert.NoError(t, gofakeit.Struct(&f))
				var kvs KVs
				kvs = kvs.AddTag("T_"+f.T1Key, f.T1)
				kvs = kvs.AddTag("T_"+f.T2Key, f.T2)
				kvs = kvs.AddTag("T_"+f.T3Key, f.T3)

				kvs = kvs.AddV2("S_"+f.SKey, f.S, true)

				kvs = kvs.AddV2("I8_"+f.I8Key, f.I8, true)
				kvs = kvs.AddV2("I16_"+f.I16Key, f.I16, true)
				kvs = kvs.AddV2("I32_"+f.I32Key, f.I32, true)
				kvs = kvs.AddV2("I64_"+f.I64Key, f.I64, true)

				kvs = kvs.AddV2("U8_"+f.U8Key, f.U8, true)
				kvs = kvs.AddV2("U16_"+f.U16Key, f.U16, true)
				kvs = kvs.AddV2("U32_"+f.U32Key, f.U32, true)
				kvs = kvs.AddV2("U64_"+f.U64Key, f.U64, true)

				kvs = kvs.AddV2("F32_"+f.F32Key, f.F32, true)
				kvs = kvs.AddV2("F64_"+f.F64Key, f.F64, true)

				kvs = kvs.AddV2("B_"+f.BKey, f.B, true)
				kvs = kvs.AddV2("D_"+f.DKey, f.D, true)

				if f.TS < 0 {
					f.TS = 0
				}

				pt := NewPointV2(f.Measurement, kvs, WithTimestamp(f.TS))

				require.Equal(t, f.T1, pt.Get("T_"+f.T1Key))
				require.Equal(t, f.T2, pt.Get("T_"+f.T2Key))
				require.Equal(t, f.T3, pt.Get("T_"+f.T3Key))

				require.Equal(t, f.S, pt.Get("S_"+f.SKey))

				require.Equal(t, int64(f.I8), pt.Get("I8_"+f.I8Key))
				require.Equalf(t, int64(f.I16), pt.Get("I16_"+f.I16Key), "got %s", pt.Pretty())
				require.Equal(t, int64(f.I32), pt.Get("I32_"+f.I32Key))
				require.Equal(t, f.I64, pt.Get("I64_"+f.I64Key))

				require.Equal(t, uint64(f.U8), pt.Get("U8_"+f.U8Key))
				require.Equal(t, uint64(f.U16), pt.Get("U16_"+f.U16Key))
				require.Equal(t, uint64(f.U32), pt.Get("U32_"+f.U32Key))
				require.Equal(t, f.U64, pt.Get("U64_"+f.U64Key))

				require.Equal(t, f.B, pt.Get("B_"+f.BKey), "got %s", pt.Pretty())
				require.Equal(t, f.D, pt.Get("D_"+f.DKey))
				require.Equalf(t, float64(f.F32), pt.Get("F32_"+f.F32Key), "got %s", pt.Pretty())
				require.Equal(t, f.F64, pt.Get("F64_"+f.F64Key))

				require.Equal(t, f.TS, pt.Time().UnixNano(), "got %s", pt.Pretty())

				if i == maxPT-1 {
					t.Logf("point: %s", pt.Pretty())
				}

				tc.pp.Put(pt)
			}

			_, err := metrics.Gather()
			assert.NoError(t, err)
		})
	}
}

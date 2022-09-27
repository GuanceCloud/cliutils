package dialtesting

import (

	//"encoding/json"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
)

const expr = `(function (h, o, u, n, d) {
    h = h[d] = h[d] || {
      q: [],
      onReady: function (c) {
        h.q.push(c)
      }
    }
    d = o.createElement(u)
    d.async = 1
    d.src = n
    n = o.getElementsByTagName(u)[0]
    n.parentNode.insertBefore(d, n)
  })(
    window,
    document,
    'script',
    'https://zhuyun-static-files-production.oss-cn-hangzhou.aliyuncs.com/browser-sdk/v2/dataflux-rum-headless.js',
    'DATAFLUX_RUM'
  )
  DATAFLUX_RUM.onReady(function () {
    DATAFLUX_RUM.init({
      trackLoadedVisibleElement: 'testheadless',
    })
  })`

var headlessCases = []struct {
	t         *HeadlessTask
	fail      bool
	reasonCnt int
}{

	{
		fail:      false,
		reasonCnt: 0,
		t: &HeadlessTask{
			ExternalID: cliutils.XID("dtst_"),
			Method:     "GET",
			URL:        "http://www.baidu.com",
			Name:       "baidu",
			Region:     "hangzhou",
			Frequency:  "1s",
			AdvanceOptions: &HeadlessAdvanceOption{
				RequestOptions: &OptRequest{
					DisableCors: true,
					// IgnoreServerCertificateError: true,
					// Proxy: "http://10.100.65.32:8080",
				},
			},
		},
	},

	{
		fail:      false,
		reasonCnt: 0,
		t: &HeadlessTask{
			ExternalID: cliutils.XID("dtst_"),
			Method:     "GET",
			URL:        "http://localhost:54321/_test_no_resp",
			Name:       "_test_no_resp",
			Region:     "hangzhou",
			Frequency:  "1s",
			AdvanceOptions: &HeadlessAdvanceOption{
				RequestOptions: &OptRequest{
					IgnoreServerCertificateError: true,
					DisableCors:                  true,
				},

				Secret: &HTTPSecret{
					NoSaveResponseBody: true,
				},
			},
		},
	},

	//
	{
		fail:      false,
		reasonCnt: 0,
		t: &HeadlessTask{
			ExternalID: cliutils.XID("dtst_"),
			Method:     "GET",
			URL:        "http://localhost:54321/_test_with_cert",
			Name:       "_test_with_cert",
			Region:     "hangzhou",
			Frequency:  "1s",
			AdvanceOptions: &HeadlessAdvanceOption{
				RequestOptions: &OptRequest{
					IgnoreServerCertificateError: true,
				},
			},
		},
	},

	// // test dial with proxy
	// {
	// 	fail:      false,
	// 	reasonCnt: 0,
	// 	t: &HeadlessTask{
	// 		ExternalID: cliutils.XID("dtst_"),
	// 		Method:     "POST",
	// 		URL:        "http://localhost:54321/_test_with_proxy",
	// 		Name:       "_test_with_proxy",
	// 		Region:     "hangzhou",
	// 		Frequency:  "1s",
	// 		AdvanceOptions: &HeadlessAdvanceOption{
	// 			RequestOptions: &OptRequest{
	// 				Proxy: `http://localhost:5432`,
	// 			},
	// 		},
	// 	},
	// },

	// test dial with headers
	{
		reasonCnt: 0,
		t: &HeadlessTask{
			ExternalID: cliutils.XID("dtst_"),
			Method:     "GET",
			URL:        "http://localhost:54321/_test_with_headers",
			Name:       "_test_with_headers",
			Region:     "hangzhou",
			Frequency:  "1s",
			AdvanceOptions: &HeadlessAdvanceOption{
				RequestOptions: &OptRequest{
					Headers: map[string]interface{}{
						"X-Header-1": "foo",
						"X-Header-2": "bar",
					},
				},
			},
		},
	},

	// test dial with auth
	{
		reasonCnt: 0,
		t: &HeadlessTask{
			ExternalID: cliutils.XID("dtst_"),
			Method:     "GET",
			URL:        "http://localhost:54321/_test_with_basic_auth",
			Name:       "_test_with_basic_auth",
			Region:     "hangzhou",
			Frequency:  "1s",
			AdvanceOptions: &HeadlessAdvanceOption{
				RequestOptions: &OptRequest{
					Auth: &HTTPOptAuth{
						Username: "foo",
						Password: "bar",
					},
				},
			},
		},
	},

	// test dial with cookie
	{
		reasonCnt: 0,
		t: &HeadlessTask{
			ExternalID: cliutils.XID("dtst_"),
			Method:     "GET",
			URL:        "http://localhost:54321/_test_with_cookie",
			Name:       "_test_with_cookie",
			Region:     "hangzhou",
			Frequency:  "1s",
			AdvanceOptions: &HeadlessAdvanceOption{
				RequestOptions: &OptRequest{
					Cookies: (&http.Cookie{
						Name:   "_test_with_cookie",
						Value:  "foo-bar",
						MaxAge: 0,
						Secure: true,
					}).String(),
				},
			},
		},
	},
}

const (
	indexHTML = `
	<!doctype html>
	<html>
	  <head>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no, viewport-fit=cover">
		<link rel="apple-touch-icon" href="https://gw.alipayobjects.com/mdn/prod_resou/afts/img/A*CUIoT4xopNYAAAAAAAAAAABkARQnAQ" />
		<meta http-equiv="X-UA-Compatible" content="edge">

		<meta name="pagetype" content="group_homepage">
		<meta name="pagename" content="group_homepage">
		<meta name="description" content="DataFlux的产品功能与最佳实践">
		<meta property="og:type" content="webpage">
		<meta property="og:title" content="DataFlux · Yuque">
		<meta property="og:url" content="https://www.yuque.com/dataflux">
		<meta property="og:description" content="DataFlux的产品功能与最佳实践">
		<meta property="og:image" content="https://cdn.nlark.com/yuque/0/2021/png/21491762/1619684399918-avatar/3fc91ab1-71b4-4548-8850-4a0097066a15.png">
		<meta name="weibo:webpage:create_at" content="2021-04-29 15:28:37">
		<meta name="weibo:webpage:update_at" content="2021-05-26 11:13:51">
		<title>DataFlux · Yuque</title>
		<link type="image/png" href="https://gw.alipayobjects.com/zos/rmsportal/UTjFYEzMSYVwzxIGVhMu.png" rel="shortcut icon" />
		<link rel="search" type="application/opensearchdescription+xml" href="/opensearch.xml" title="语雀" />
		<link rel="manifest" href="/manifest.json" />
		<meta name="theme_color" content="#05192D" />

		  <link rel="stylesheet" href="https://gw.alipayobjects.com/os/chair-script/skylark/common.d25726e4.chunk.css" />

		<link rel="stylesheet" href="https://gw.alipayobjects.com/os/chair-script/skylark/pc.3c59fc2b.css" />
		<link href="https://gw.alipayobjects.com" rel="dns-prefetch" />
	<link href="https://mdap.alipay.com" rel="dns-prefetch" />
	<link href="https://cdn.nlark.com" rel="dns-prefetch" />
	<link href="https://cdn.yuque.com" rel="dns-prefetch" />
	<link href="https://kcart.alipay.com" rel="dns-prefetch" />
	<link href="https://cdn-pri.nlark.com" rel="dns-prefetch" />
	<link href="https://g.yuque.com" rel="dns-prefetch" />
	<link href="https://mdap.yuque.com" rel="dns-prefetch" />
	<meta name="baidu-site-verification" content="WGwq1qW6TC" />
	<meta name="renderer" content="webkit">
	</head>
	<body>
	    <div id="result">%s</div>
		<script crossorigin src="https://gw.alipayobjects.com/os/chair-script/skylark/pc.06020c14.js"></script>
		<script crossorigin src="https://gw.alipayobjects.com/os/lib/alipay/yuyan-monitor-web/2.0.29/dist/index.umd.min.js"></script>
	  </body>
	</html>

	`
)

// cookieServer creates a simple HTTP server that logs any passed cookies.
func testServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/_test_with_cookie", func(res http.ResponseWriter, req *http.Request) {
		cookies := req.Cookies()
		for i, cookie := range cookies {
			log.Printf("server:  from %s, server received cookie %d: %v", req.RemoteAddr, i, cookie)
		}
		buf, err := json.MarshalIndent(req.Cookies(), "", "  ")
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(res, indexHTML, string(buf))
	})

	mux.HandleFunc("/_test_with_headers", func(res http.ResponseWriter, req *http.Request) {
		buf, err := json.MarshalIndent(req.Header, "", "  ")
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		h1 := req.Header.Get(`X-Header-1`)
		h2 := req.Header.Get(`X-Header-2`)
		if h1 != `foo` || h2 != `bar` {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		// fmt.Printf(`header %s`, string(buf))
		fmt.Fprintf(res, indexHTML, string(buf))
	})

	mux.HandleFunc("/_test_with_basic_auth", func(res http.ResponseWriter, req *http.Request) {
		user, pwd, ok := req.BasicAuth()
		if !ok {
			fmt.Printf(`basic auth failed`)
			buf, _ := json.MarshalIndent(req.Header, "", "  ")
			fmt.Printf(`basic auth failed %s`, string(buf))

			// fmt.Fprintf(res, indexHTML, "basic auth failed")
			http.Error(res, "basic auth failed", http.StatusInternalServerError)
			return
		}
		buf := fmt.Sprintf("user: %s, password: %s", user, pwd)
		fmt.Fprintf(res, indexHTML, buf)
	})

	//_test_with_cert
	mux.HandleFunc("/_test_with_cert", func(res http.ResponseWriter, req *http.Request) {
		buf := fmt.Sprintf("request tls: %+#v", req.TLS)
		fmt.Fprintf(res, indexHTML, buf)
	})

	mux.HandleFunc("/_test_no_resp", func(res http.ResponseWriter, req *http.Request) {
		buf := fmt.Sprintf("_test_no_resp:xxxx")
		fmt.Fprintf(res, indexHTML, buf)
	})

	return http.ListenAndServe(addr, mux)
}

func TestHeadless(t *testing.T) {
	flagPort := flag.Int("port", 54321, "port")
	flag.Parse()

	// start test server
	go func() {
		if err := testServer(fmt.Sprintf(":%d", *flagPort)); err != nil {
			t.Error(err)
		}
	}()

	for i, c := range headlessCases {
		// if i != 0 {
		// 	continue
		// }

		if err := c.t.Init(); err != nil {
			if c.fail == false {
				t.Errorf("case %s failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		log.Printf(`headless case run %d`, i)
		if err := c.t.Run(); err != nil {
			if c.fail == false {
				t.Errorf("case %s failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		t.Logf("linedata: %s \n ", c.t.GetLineData())

		if strings.Contains(c.t.GetLineData(), `XMLHttpRequest`) {
			t.Errorf("linedata XMLHttpRequest \n ")
		}
	}
}

func TestChromedp(t *testing.T) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-web-security", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// create context
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	actions := []chromedp.Action{}
	actions = append(actions, chromedp.Navigate("http://www.baidu.com"))
	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		_, exp, err := runtime.Evaluate(expr).Do(ctx)
		if err != nil {
			return err
		}
		if exp != nil {
			return exp
		}
		return nil
	}))

	var res []interface{}
	actions = append(actions, chromedp.WaitVisible("#testheadless"))
	actions = append(actions, chromedp.Evaluate("window.DATAFLUX_RUM && DATAFLUX_RUM.getInternalData()", &res))

	err := chromedp.Run(ctx, actions...)
	if err != nil {
		log.Fatal(err)
	}

	data, _ := json.Marshal(res)

	fmt.Printf("evaluate: %s\n", string(data))

	// for _, s := range res {
	// 	fmt.Printf("struct: %+#v\n", s)
	// }
	// fmt.Printf("Document tree: ")
	// fmt.Print(nodes[0].Dump("  ", "  ", false))
}

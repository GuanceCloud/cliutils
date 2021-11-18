package ws

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
)

var (
	__wsip     = `0.0.0.0`
	__wsport   = 54321
	__wsupath  = "/wstest"
	__df_wsurl = url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", __wsip, __wsport), Path: __wsupath}

	__fcliCnt   = flag.Int("cli-cnt", 128, ``)
	__ftestTime = flag.Duration("test-time", time.Minute, ``)

	__wg = sync.WaitGroup{}
)

func TestWSServer(t *testing.T) {
	flag.Parse()

	// ws server
	dfwsurl := fmt.Sprintf("%s:%d", __wsip, __wsport)
	df_srv, err := NewServer(dfwsurl, __wsupath)
	if err != nil {
		t.Fatal(err)
	}

	// msg-handle callback
	df_srv.MsgHandler = func(s *Server, c net.Conn, data []byte, op ws.OpCode) error {
		SendMsgToClient([]byte(fmt.Sprintf("your are %s", c.RemoteAddr().String())), c)
		return nil
	}

	// add-cli callback
	df_srv.AddCli = func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			l.Error("ws.UpgradeHTTP error: %s", err.Error())
			return
		}

		id := r.URL.Query().Get("id") // get id from ws request URL
		if id == "" {
			t.Fatal("id miss")
		}

		if err := df_srv.AddConnection(conn); err != nil {
			l.Error(err)
		}
		return
	}

	go df_srv.Start()
	time.Sleep(time.Second)

	type wscli struct {
		id  string
		cli *websocket.Conn
	}

	ncli := *__fcliCnt

	// datakit as ws proxy client
	dkclis := []*wscli{}
	for i := 0; i < ncli; i++ {

		cliid := cliutils.XID("id_")

		dw_wsurl := url.URL{
			Scheme:   "ws",
			Host:     fmt.Sprintf("%s:%d", __wsip, __wsport),
			Path:     __wsupath,
			RawQuery: fmt.Sprintf(`id=%s&version=v1.0.0.0-1234-gabcdef`, cliid),
		}

		dk_cli, _, err := websocket.DefaultDialer.Dial(dw_wsurl.String(), nil)
		if err != nil {
			t.Fatalf("Failed to connect: %s", err.Error())
		}
		dkclis = append(dkclis, &wscli{id: cliid, cli: dk_cli})
	}

	__wg.Add(ncli)
	ch := make(chan interface{})

	// ws-cli send msg to ws-server
	for i := 0; i < ncli; i++ {
		go func(i int) {
			total := 0
			c := dkclis[i]
			defer __wg.Done()

			for {
				if err := c.cli.WriteMessage(websocket.TextMessage, []byte(c.id)); err != nil {
					t.Errorf("client write failed: %s", err.Error())
				}

				total++
				if _, resp, err := c.cli.ReadMessage(); err != nil {
					_ = resp
					t.Log(err)
				} else {
					if total%(ncli/2) == 0 {
						l.Debugf("%s", string(resp))
					}
				}

				time.Sleep(time.Millisecond)
				select {
				case <-ch:
					c.cli.Close()
					l.Debugf("cli %d exit", i)
					return
				default:
				}
			}
		}(i)
	}

	time.Sleep(*__ftestTime)
	close(ch)

	df_srv.Stop()

	__wg.Wait()
}

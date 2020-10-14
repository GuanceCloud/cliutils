package ws

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	"github.com/koding/websocketproxy"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
)

var (
	__wsip     = `0.0.0.0`
	__wsport   = 18080
	__wsupath  = "/wstest"
	__df_wsurl = url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", __wsip, __wsport+1), Path: __wsupath}

	__wg = sync.WaitGroup{}
)

type cli struct {
	id   string
	conn net.Conn
}

func (c *cli) ID() string {
	return c.id
}

func (c *cli) Conn() net.Conn {
	return c.conn
}

func TestProxy(t *testing.T) {

	// dataflux as ws server
	dfwsurl := fmt.Sprintf("%s:%d", __wsip, __wsport+1)
	df_srv, err := NewServer(dfwsurl, __wsupath)
	if err != nil {
		t.Fatal(err)
	}

	df_srv.MsgHandler = func(s *Server, c net.Conn, data []byte, op ws.OpCode) error {
		s.SendServerMsg([]byte(fmt.Sprintf("your are %s", c.RemoteAddr().String())), []string{string(data)}...)
		return nil
	}

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

		l.Debugf("request URL: %s", r.URL.String())

		cli := &cli{
			conn: conn,
			id:   id,
		}

		if err := df_srv.AddClient(cli); err != nil {
			l.Error(err)
		}
		return
	}

	go df_srv.Start()
	time.Sleep(time.Second)

	// dw ws proxy
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", __wsport), websocketproxy.NewProxy(&__df_wsurl)); err != nil {
			t.Fatal(err)
		}
	}()

	time.Sleep(time.Second)

	ncli := 2

	type wscli struct {
		id  string
		cli *websocket.Conn
	}

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

	time.Sleep(time.Second)

	for _, c := range df_srv.clis {
		l.Debugf("dk-ws-cli: %+#v", c)
	}

	__wg.Add(ncli)
	ch := make(chan interface{})

	for i := 0; i < ncli; i++ {
		go func(i int) {
			total := 0
			c := dkclis[i]
			defer __wg.Done()

			for {
				if err := c.cli.WriteMessage(websocket.TextMessage, []byte(c.id)); err != nil {
					t.Fatalf("client write failed: %s", err.Error())
				}

				total++
				if _, resp, err := c.cli.ReadMessage(); err != nil {
					t.Log(err)
				} else {
					if total%512 == 0 {
						l.Debugf("%s", string(resp))
					}
				}

				time.Sleep(time.Millisecond)
				select {
				case <-ch:
					c.cli.Close()
					return
				default:
				}
			}
		}(i)
	}

	time.Sleep(time.Minute)
	close(ch)

	df_srv.Stop()

	__wg.Wait()
}

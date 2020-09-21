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
)

var (
	__wsip     = `0.0.0.0`
	__wsport   = 18080
	__wsupath  = "/wstest"
	__df_wsurl = url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", __wsip, __wsport+1), Path: __wsupath}
	__dw_wsurl = url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", __wsip, __wsport), Path: __wsupath}

	__wg = sync.WaitGroup{}
)

func TestProxy(t *testing.T) {

	// dataflux as ws server
	dfwsurl := fmt.Sprintf("%s:%d", __wsip, __wsport+1)
	df_srv, err := NewServer(dfwsurl, __wsupath, func(s *Server, c net.Conn, data []byte, op ws.OpCode) error {
		s.SendServerMsg(c.RemoteAddr().String(), []byte(fmt.Sprintf("your are %s", c.RemoteAddr().String())))
		return nil
	})
	if err != nil {
		t.Fatal(err)
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

	// datakit as ws proxy client
	dkclis := []*websocket.Conn{}
	for i := 0; i < 100; i++ {
		dk_cli, _, err := websocket.DefaultDialer.Dial(__dw_wsurl.String(), nil)
		if err != nil {
			t.Fatalf("Failed to connect: %s", err.Error())
		}
		dkclis = append(dkclis, dk_cli)
	}

	time.Sleep(time.Second)

	for _, c := range df_srv.clis {
		l.Debugf("dk-ws-cli: %+#v", c)
	}

	__wg.Add(100)
	ch := make(chan interface{})

	for i := 0; i < 100; i++ {
		go func(i int) {
			total := 0
			c := dkclis[i]
			defer __wg.Done()

			for {
				if err := c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("who am i[%d]", i))); err != nil {
					t.Fatalf("client write failed: %s", err.Error())
				}

				total++
				if _, resp, err := c.ReadMessage(); err != nil {
					t.Log(err)
				} else {
					if total%512 == 0 {
						l.Debugf("%s", string(resp))
					}
				}

				time.Sleep(time.Millisecond)
				select {
				case <-ch:
					c.Close()
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

func TestServer2(t *testing.T) {

	// clients
	nconn := 1024 * 60

	var conns []*websocket.Conn
	for i := 0; i < nconn; i++ {
		c, _, err := websocket.DefaultDialer.Dial(__dw_wsurl.String(), nil)
		if err != nil {
			fmt.Println("Failed to connect", i, err)
			break
		}
		conns = append(conns, c)
		defer func() {
			c.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second))
			time.Sleep(time.Second)
			c.Close()
		}()
	}

	fmt.Printf("Finished initializing %d connections\n", len(conns))

	totalSend := 0
	for {
		for i := 0; i < len(conns); i++ {
			time.Sleep(time.Duration(totalSend%7) * time.Microsecond)
			conn := conns[i]
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*5)); err != nil {
				fmt.Printf("Failed to receive pong: %v", err)
			}
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Hello from conn %v", i)))
			totalSend++
		}
	}
}

package ws

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	//"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
)

var (
	__wsip    = `0.0.0.0`
	__wsport  = ":18080"
	__wsupath = "/wstest"
	__wsurl   = url.URL{Scheme: "ws", Host: __wsip + __wsport, Path: __wsupath}
	__wg      = sync.WaitGroup{}

	__ASK = MsgType(0)
	__ANS = MsgType(1)
)

type testmsg struct {
	MsgType MsgType `json:"msg_type"`
	MsgData string  `json:"msg_data"`
	ID      string  `json:"id,omitempty"`
	TraceID string  `json:"trace_id"`

	resp chan interface{}
}

func (tm *testmsg) Type() MsgType            { return tm.MsgType }
func (tm *testmsg) Msg() interface{}         { return tm.MsgData }
func (tm *testmsg) To() string               { return tm.ID }
func (tm *testmsg) TraceID() string          { return tm.TraceID }
func (tm *testmsg) SetTraceID(id string)     { tm.TraceID = id }
func (tm *testmsg) GetResp() interface{}     { return <-tm.resp }
func (tm *testmsg) SetResp(resp interface{}) { tm.resp <- resp }
func (tm *testmsg) Expired() bool            { return false }

func (tm *testmsg) Data() []byte {
	j, err := json.Marshal(tm)
	if err != nil {
		panic(err)
	}

	return j
}

func handler(s *Server, c net.Conn, data []byte, op ws.OpCode) error {

	var tm testmsg
	if err := json.Unmarshal(data, &tm); err != nil {
		return err
	}

	l.Debugf("receive from %s(op: %d): %s", c.RemoteAddr().String(), op, string(data))
	switch tm.MsgType {
	case __ASK:

		ans := &testmsg{
			MsgData: fmt.Sprintf("your addr is %s, what's you name?", c.RemoteAddr().String()),
			ID:      c.RemoteAddr().String(),
			MsgType: __ASK,
		}

		resp, err := s.SendServerMsg(ans, true, 0)
		if err != nil {
			return err
		}

		switch resp.(type) {
		case testmsg:
			l.Debugf("%+#v", tm.(testmsg))
		default:
			panic("unknown msg")
		}

	case __ANS:
		l.Debugf("%+#v", tm)
	}

	return nil
}

func TestServer1(t *testing.T) {
	s, err := NewServer(__wsip+__wsport, __wsupath, handler)
	if err != nil {
		t.Fatal(err)
	}

	go s.Start()

	time.Sleep(time.Second)

	c, _, err := websocket.DefaultDialer.Dial(__wsurl.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %s", err.Error())
	}

	//if err := c.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
	//	t.Fatal(err)
	//}

	if err := c.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)); err != nil {
		t.Fatalf("client write ping failed: %s", err.Error())
	}

	j, _ := json.Marshal(&testmsg{
		MsgType: __ASK,
		MsgData: "where am I?",
	})

	if err := c.WriteMessage(websocket.TextMessage, j); err != nil {
		t.Fatalf("client write msg failed: %s", err.Error())
	} else {
		if mt, resp, err := c.ReadMessage(); err != nil {
			t.Fatalf("client read msg failed: %s", err.Error())
		} else {
			l.Debugf("write ok, resp: %s(%d)", string(resp), mt)
			var tm testmsg
			if err := json.Unmarshal(resp, &tm); err != nil {
				t.Fatal(err)
			} else {
				j, _ = json.Marshal(&testmsg{
					MsgType: __ANS,
					MsgData: "I'm 42",
				})
			}
		}
	}

	s.Stop()

	for i := 0; i < 10; i++ {
		if err := c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Hello from client[%d]", i))); err != nil {
			t.Fatalf("client write failed: %s", err.Error())
		} else {
			//time.Sleep(time.Millisecond)
			l.Debugf("write %d ok", i)
		}
	}
}

func TestServer2(t *testing.T) {

	// clients
	nconn := 1024 * 60

	var conns []*websocket.Conn
	for i := 0; i < nconn; i++ {
		c, _, err := websocket.DefaultDialer.Dial(__wsurl.String(), nil)
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

/*
func testWSServer() {

	__wg.Add(1)
	go func() {
		defer __wg.Done()
		Start(__wsip+__wsport, __wsupath)
	}()

	time.Sleep(time.Second)
}

func testcliexit(c *websocket.Conn) {
	c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	c.Close()
}

func testonline(c *websocket.Conn, id string) {
	payload, err := json.Marshal(
		&onlineMsg{
			Version: "ver-xxx",
			OS:      "os-xxx",
			Arch:    "arch-xxx",
			Name:    "name-xxx",
			Uptime:  time.Now(),
		},
	)

	msg := &datakitMsg{
		Type:    MsgOnline,
		UUID:    id,
		Payload: payload,
	}

	j, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	c.WriteMessage(websocket.TextMessage, j)
}

func TestConfigDatakit(t *testing.T) {
	testWSServer()
	c, _, err := websocket.DefaultDialer.Dial(__wsurl.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %s", err.Error())
	}

	defer testcliexit(c)

	dkid := cliutils.XID("dk_")

	__wg.Add(1)
	go func() {
		defer __wg.Done()

		testonline(c, dkid)

		_, data, err := c.ReadMessage()
		if err != nil {
			t.Log(err)
			return
		}

		t.Log(string(data))

		dkmsg := datakitMsg{}

		if err := json.Unmarshal(data, &dkmsg); err != nil {
			t.Fatal(err)
		}

		dkmsg.Resp = "ok" // response ok

		j, err := json.Marshal(dkmsg)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("resp %s", string(j))

		c.WriteMessage(websocket.TextMessage, j)
	}()

	time.Sleep(time.Second)

	payload, err := json.Marshal(
		[]*configMsg{
			&configMsg{
				Input:  "intput-xxx",
				Config: "base64(toml)",
			},
			&configMsg{
				Input:  "intput-yyy",
				Config: "base64(toml)",
			},
		},
	)

	msg := &datakitMsg{
		Type:    MsgConfig,
		UUID:    dkid,
		Payload: payload,
	}

	resp, err := DispatchMsg(msg)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)

	testcliexit(c)
	Stop()
	__wg.Wait()
}

func TestDatakitOnline(t *testing.T) {
	testWSServer()

	c, _, err := websocket.DefaultDialer.Dial(__wsurl.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %s", err.Error())
	}

	defer testcliexit(c)

	dkid := cliutils.XID("dk_")
	testonline(c, dkid)

	hbmsg := &datakitMsg{
		Type: MsgHeartbeat,
		UUID: dkid,
	}

	j, err := json.Marshal(hbmsg)
	if err != nil {
		t.Fatal(err)
	}

	c.WriteMessage(websocket.TextMessage, j)
	time.Sleep(time.Second)

	testcliexit(c)
	Stop()
	__wg.Wait()
} */

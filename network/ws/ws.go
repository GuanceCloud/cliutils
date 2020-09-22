package ws

import (
	"context"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"syscall"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/logger"
)

var (
	l = logger.DefaultSLogger("ws")
)

type srvmsg struct {
	to  []string
	msg []byte
}

type Server struct {
	Path    string
	Bind    string
	ChanCap int

	hbinterval time.Duration

	handler func(*Server, net.Conn, []byte, ws.OpCode) error //server msg handler
	uptime  time.Time

	clis map[string]*Cli

	exit *cliutils.Sem
	wg   *sync.WaitGroup

	sendMsgCh chan *srvmsg

	hbCh    chan string
	wscliCh chan *Cli

	epoller *epoll
}

func (s *Server) SetMaxHeartbeatInterval(i time.Duration) {
	s.hbinterval = i
}

func NewServer(bind, path string, h func(*Server, net.Conn, []byte, ws.OpCode) error) (s *Server, err error) {

	s = &Server{
		Path: path,
		Bind: bind,

		ChanCap: CommonChanCap,
		handler: h,

		uptime: time.Now(),

		clis: map[string]*Cli{},
		exit: cliutils.NewSem(),
		wg:   &sync.WaitGroup{},

		sendMsgCh: make(chan *srvmsg, CommonChanCap),
		hbCh:      make(chan string, CommonChanCap),
		wscliCh:   make(chan *Cli, CommonChanCap),
	}

	s.epoller, err = MkEpoll()
	if err != nil {
		l.Error("MkEpoll() error: %s", err.Error())
		return
	}

	return
}

func (s *Server) epollAddConn(w http.ResponseWriter, r *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		l.Error("ws.UpgradeHTTP error: %s", err.Error())
		return
	}

	if err := s.epoller.Add(conn); err != nil {
		l.Errorf("epoll.Add() error: %s", err.Error())
		conn.Close()
		return
	}

	cli := &Cli{
		conn: conn,
		id:   conn.RemoteAddr().String(), // FIXME:
		born: time.Now(),
	}

	l.Debugf("epoll add connection from %s", conn.RemoteAddr().String())
	s.wscliCh <- cli
}

func (s *Server) Stop() {
	s.exit.Close()

	l.Debug("wait...")
	s.wg.Wait()

	l.Debug("wait done")
}

func (s *Server) Start() {

	// remove resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	// Enable pprof hooks
	//go func() {
	//	if err := http.ListenAndServe("0.0.0.0:6060", nil); err != nil {
	//		l.Fatalf("pprof failed: %v", err)
	//	}
	//}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.dispatcher()
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.startEpoll()
	}()

	srv := &http.Server{
		Addr: s.Bind,
	}

	http.HandleFunc(s.Path, s.epollAddConn)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := srv.ListenAndServe(); err != nil {
			l.Info(err)
		}
	}()

	<-s.exit.Wait()
	if err := srv.Shutdown(context.TODO()); err != nil {
		l.Errorf("srv.Shutdown: %s", err.Error())
	}

	l.Info("websocket server stopped.")
}

func (s *Server) startEpoll() {
	for {

		select {
		case <-s.exit.Wait():
			l.Debug("epoll exit.")
			s.epoller.Close()
			return

		default:

			//l.Debug("eneneneneen")

			connections, err := s.epoller.Wait() // wait for 100ms
			if err != nil {
				l.Errorf("Failed to epoll wait %v", err)
				continue
			}

			for _, conn := range connections {

				if conn == nil {
					break
				}

				if data, opcode, err := wsutil.ReadClientData(conn); err != nil {
					if err := s.epoller.Remove(conn); err != nil {
						l.Errorf("Failed to remove %v", err)
					}

					l.Debugf("close cli %s", conn.RemoteAddr().String())
					conn.Close()
				} else {
					if err := s.handler(s, conn, data, opcode); err != nil {
						l.Error("s.handler() error: %s", err.Error())
					}
				}
			}
		}
	}
}

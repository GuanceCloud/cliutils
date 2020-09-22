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
	l             = logger.DefaultSLogger("ws")
	CommonChanCap = 128
)

type Cli interface {
	ID() string
	Conn() net.Conn
}

type srvmsg struct {
	to  []string
	msg []byte
}

type Server struct {
	Path    string
	Bind    string
	ChanCap int

	hbinterval time.Duration

	MsgHandler func(*Server, net.Conn, []byte, ws.OpCode) error //server msg handler
	AddCli     func(w http.ResponseWriter, r *http.Request)

	uptime time.Time

	clis map[string]Cli

	exit *cliutils.Sem
	wg   *sync.WaitGroup

	sendMsgCh chan *srvmsg
	wscliCh   chan Cli

	epoller *epoll
}

func (s *Server) SetMaxHeartbeatInterval(i time.Duration) {
	s.hbinterval = i
}

func NewServer(bind, path string) (s *Server, err error) {

	s = &Server{
		Path: path,
		Bind: bind,

		ChanCap: CommonChanCap,

		uptime: time.Now(),

		clis: map[string]Cli{},
		exit: cliutils.NewSem(),
		wg:   &sync.WaitGroup{},

		sendMsgCh: make(chan *srvmsg, CommonChanCap),
		wscliCh:   make(chan Cli, CommonChanCap),
	}

	s.epoller, err = MkEpoll()
	if err != nil {
		l.Error("MkEpoll() error: %s", err.Error())
		return
	}

	return
}

func (s *Server) AddClient(cli Cli) error {

	l.Debugf("epoll add connection from %s", cli.Conn().RemoteAddr().String())
	if err := s.epoller.Add(cli.Conn()); err != nil {
		l.Errorf("epoll.Add() error: %s", err.Error())
		cli.Conn().Close()
		return err
	}

	s.wscliCh <- cli
	return nil
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

	if s.AddCli == nil {
		l.Fatal("AddCli not set")
	}

	http.HandleFunc(s.Path, s.AddCli)

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
					if s.MsgHandler != nil {
						if err := s.MsgHandler(s, conn, data, opcode); err != nil {
							l.Error("s.handler() error: %s", err.Error())
						}
					}
				}
			}
		}
	}
}

package server

import (
	"log"
	"net"
	"sync"

	"golang.org/x/net/netutil"
)

// Server 定義 Server 的一些參數
type Server struct {
	// 自定義 Tag
	Tag interface{}
	// MaxConnections 定義最大連接數量，必須大於 0 , 否則無上限
	MaxConnections int
	// BindAddress 定義要 listen 的 Address 及 Port , 如 127.0.0.1:8000
	BindAddress string
	listener    net.Listener

	shutdownChan chan bool // 此值如果為 true , 代表 Server 必須停止，所有工作都需要關閉
	runningCount int
	runningMutex sync.Mutex
}

// Conn 是當 Accept 後產生的連線物件
type Conn struct {
	net.Conn // 繼承原本的 net.Conn
	ctx      interface{}
	server   *Server // Server
}

// SetContext 設定任意型態的關聯資源 , 可藉由 Context() 取得
func (c *Conn) SetContext(ctx interface{}) {
	c.ctx = ctx
}

// Context 取得關聯資源 , 可藉由 SetContext() 設定
func (c *Conn) Context() interface{} {
	return c.ctx
}

// Server ...
func (c *Conn) Server() *Server { return c.server }

// Serve ...
func (s *Server) Serve(event Event) error {
	var err error
	s.runningCount = 0
	s.listener, err = net.Listen("tcp", s.BindAddress)

	if err != nil {
		return err
	}
	s.listener = netutil.LimitListener(s.listener, s.MaxConnections)

	var nextAction Action
	nextAction = None

	if event.OnStartup != nil {
		// 如果有實作 Startup 事件，就呼叫
		// TODO next action ? only allow Shutdown
		nextAction = event.OnStartup(s)
	}

	if &nextAction == nil || nextAction == None {
		return s.loopAccept(event)
	}

	return nil
}

// loopAccept 開始接受外部連線
func (s *Server) loopAccept(event Event) error {

	s.shutdownChan = make(chan bool, 1)

	for {

		select {
		default:
		case <-s.shutdownChan:
			s.triggerOnShutdown(event)
			return nil
		}

		netconn, err := s.listener.Accept()
		if err == nil {

			conn := &Conn{netconn, nil, s}
			s.runningMutex.Lock()
			s.runningCount++
			if s.runningCount < s.MaxConnections {
				s.runningMutex.Unlock()
			}

			go func(c *Conn) {

				nextAction := s.triggerOnConnect(event, c)
				switch nextAction {
				case Close:
					s.triggerOnDisconnect(event, c)
				case Shutdown:
					s.Shutdown()

				}

				if s.runningCount >= s.MaxConnections {
					s.runningCount--
					if s.runningCount < s.MaxConnections {
						s.runningMutex.Unlock()
					}
				} else {
					s.runningMutex.Lock()
					s.runningCount--
					s.runningMutex.Unlock()
				}

			}(conn)
		} else {
			if netconn != nil {
				netconn.Close()
			}
			log.Printf("Accept error : %s\n", err.Error())
			return err
		}

	}

}

func (s *Server) triggerOnConnect(event Event, c *Conn) Action {
	nextAction := Close
	if event.OnConnect != nil {
		nextAction := event.OnConnect(c)
		if &nextAction == nil {
			nextAction = Close
		}
	}
	return nextAction
}

func (s *Server) triggerOnDisconnect(event Event, c *Conn) Action {
	nextAction := None
	c.Close() // 關閉連線
	if event.OnDisconnect != nil {
		nextAction = event.OnDisconnect(c)
		if &nextAction == nil {
			nextAction = None
		}
	}
	return nextAction
}

func (s *Server) triggerOnShutdown(event Event) {
	if event.OnShutdown != nil {
		event.OnShutdown(s)
	}
}

// Shutdown 停止服務
func (s *Server) Shutdown() {
	close(s.shutdownChan)
	s.listener.Close()
}

// Action 定義 Server 接下來的動作
type Action int

const (
	// None action , next default event will be trigger
	None Action = iota
	// Close client connection
	Close
	// Shutdown server
	Shutdown
)

// Event 啟動 Server 時必須實作以下事件
type Event struct {
	// Startup 代表 Server 啟動事件
	OnStartup func(*Server) (action Action)
	// OnConnect 當有 Client 連線成功時觸發 , 所有邏輯可以寫在這
	OnConnect func(*Conn) (action Action)
	// OnDisconnec Client 斷線時觸發
	OnDisconnect func(*Conn) (action Action)
	// OnShutdown Server 要停止前觸發
	OnShutdown func(*Server)
}

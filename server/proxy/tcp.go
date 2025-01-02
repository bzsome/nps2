package proxy

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server/connection"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type TunnelModeServer struct {
	BaseServer
	process           process
	listener          net.Listener
	stopChan          chan struct{} // 停止通道
	activeConnections sync.Map      // 连接池
}

// tcp|http|host
func NewTunnelModeServer(process process, bridge NetBridge, task *file.Tunnel) *TunnelModeServer {
	allowLocalProxy, _ := beego.AppConfig.Bool("allow_local_proxy")
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	s.allowLocalProxy = allowLocalProxy
	s.stopChan = make(chan struct{}) // 初始化停止通道
	s.activeConnections = sync.Map{} // 初始化连接池
	return s
}

// 开始
func (s *TunnelModeServer) Start() error {
	return conn.NewTcpListenerAndProcess(s.task.ServerIp+":"+strconv.Itoa(s.task.Port), func(c net.Conn) {
		// 将新连接加入到连接池中
		s.activeConnections.Store(c, struct{}{})

		defer func() {
			// 从连接池中移除连接
			s.activeConnections.Delete(c)

			if c != nil {
				c.Close()
			}
		}()

		select {
		case <-s.stopChan: // 接收到停止信号时关闭连接
			logs.Info("Connection closed due to configuration change")
			c.Close()
			return
		default:
			if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
				logs.Warn("client id %d, task id %d, error %s, when tcp connection", s.task.Client.Id, s.task.Id, err.Error())
				c.Close()
				return
			}

			logs.Trace("new tcp connection, local port %d, client %d, remote address %s", s.task.Port, s.task.Client.Id, c.RemoteAddr())

			err := s.process(conn.NewConn(c), s)
			if err == nil {
				s.task.Client.AddConn()
			}
		}
	}, &s.listener)
}

// Close 停止服务器并关闭所有连接
func (s *TunnelModeServer) Close() error {
	// 发送停止信号，通知所有连接断开
	close(s.stopChan)

	// 遍历连接池中的所有连接并关闭它们
	s.activeConnections.Range(func(key, value interface{}) bool {
		if conn, ok := key.(net.Conn); ok {
			conn.Close()
		}
		return true
	})

	// 关闭监听器
	return s.listener.Close()
}

// web管理方式
type WebServer struct {
	BaseServer
}

// 开始
func (s *WebServer) Start() error {
	p, _ := beego.AppConfig.Int("web_port")
	if p == 0 {
		stop := make(chan struct{})
		<-stop
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.SetStaticPath(beego.AppConfig.String("web_base_url")+"/static", filepath.Join(common.GetRunPath(), "web", "static"))
	beego.SetViewsPath(filepath.Join(common.GetRunPath(), "web", "views"))
	err := errors.New("Web management startup failure ")
	var l net.Listener
	if l, err = connection.GetWebManagerListener(); err == nil {
		beego.InitBeforeHTTPRun()
		if beego.AppConfig.String("web_open_ssl") == "true" {
			keyPath := beego.AppConfig.String("web_key_file")
			certPath := beego.AppConfig.String("web_cert_file")
			err = http.ServeTLS(l, beego.BeeApp.Handlers, certPath, keyPath)
		} else {
			err = http.Serve(l, beego.BeeApp.Handlers)
		}
	} else {
		logs.Error(err)
	}
	return err
}

func (s *WebServer) Close() error {
	return nil
}

// new
func NewWebServer(bridge *bridge.Bridge) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

type process func(c *conn.Conn, s *TunnelModeServer) error

// tcp proxy
func ProcessTunnel(c *conn.Conn, s *TunnelModeServer) error {
	targetAddr, err := s.task.Target.GetRandomTarget()
	if err != nil {
		c.Close()
		logs.Warn("tcp port %d ,client id %d,task id %d connect error %s", s.task.Port, s.task.Client.Id, s.task.Id, err.Error())
		return err
	}

	return s.DealClient(c, s.task.Client, targetAddr, nil, common.CONN_TCP, nil, s.task.Client.Flow, s.task.Target.ProxyProtocol, s.task.Target.LocalProxy, s.task)
}

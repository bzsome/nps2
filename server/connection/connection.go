package connection

import (
	"net"
	"os"
	"strconv"

	"ehang.io/nps/lib/pmux"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

var pMux *pmux.PortMux
var bridgePort string
var httpPort string
var webPort string

func InitConnectionService() {
	bridgePort = beego.AppConfig.String("bridge_port")
	webPort = beego.AppConfig.String("web_port")

	if httpPort == bridgePort || webPort == bridgePort {
		port, err := strconv.Atoi(bridgePort)
		if err != nil {
			logs.Error(err)
			os.Exit(0)
		}
		pMux = pmux.NewPortMux(port, beego.AppConfig.String("web_host"))
	}
}

func GetBridgeListener(tp string) (net.Listener, error) {
	logs.Info("server start, the bridge type is %s, the bridge port is %s", tp, bridgePort)
	var p int
	var err error
	if p, err = strconv.Atoi(bridgePort); err != nil {
		return nil, err
	}
	if pMux != nil {
		return pMux.GetClientListener(), nil
	}
	return net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP(beego.AppConfig.String("bridge_ip")), p, ""})
}

func GetWebManagerListener() (net.Listener, error) {
	if pMux != nil && webPort == bridgePort {
		logs.Info("Web management start, access port is", bridgePort)
		return pMux.GetManagerListener(), nil
	}
	logs.Info("web management start, access port is", webPort)
	return getTcpListener(beego.AppConfig.String("web_ip"), webPort)
}

func getTcpListener(ip, p string) (net.Listener, error) {
	port, err := strconv.Atoi(p)
	if err != nil {
		logs.Error(err)
		os.Exit(0)
	}
	if ip == "" {
		ip = "0.0.0.0"
	}
	return net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP(ip), port, ""})
}

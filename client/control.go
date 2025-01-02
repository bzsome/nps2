package client

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/version"
	"github.com/astaxie/beego/logs"
	"github.com/xtaci/kcp-go"
	"golang.org/x/net/proxy"
)

var tlsEnable1 = false

func SetTlsEnable(tlsEnable11 bool) {
	tlsEnable1 = tlsEnable11
}

func GetTlsEnable() bool {
	return tlsEnable1
}

func GetTaskStatus(path string) {
	cnf, err := config.NewConfig(path)
	if err != nil {
		log.Fatalln(err)
	}
	c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
	if err != nil {
		log.Fatalln(err)
	}
	if _, err := c.Write([]byte(common.WORK_STATUS)); err != nil {
		log.Fatalln(err)
	}
	//read now vKey and write to server
	if f, err := common.ReadAllFromFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt")); err != nil {
		log.Fatalln(err)
	} else if _, err := c.Write([]byte(crypt.Md5(string(f)))); err != nil {
		log.Fatalln(err)
	}
	var isPub bool
	binary.Read(c, binary.LittleEndian, &isPub)
	if l, err := c.GetLen(); err != nil {
		log.Fatalln(err)
	} else if b, err := c.GetShortContent(l); err != nil {
		log.Fatalln(err)
	} else {
		arr := strings.Split(string(b), common.CONN_DATA_SEQ)
		for _, v := range cnf.Tasks {
			ports := common.GetPorts(v.Ports)
			if v.Mode == "secret" {
				ports = append(ports, 0)
			}
			for _, vv := range ports {
				var remark string
				if len(ports) > 1 {
					remark = v.Remark + "_" + strconv.Itoa(vv)
				} else {
					remark = v.Remark
				}
				if common.InStrArr(arr, remark) {
					log.Println(remark, "ok")
				} else {
					log.Println(remark, "not running")
				}
			}
		}
	}
	os.Exit(0)
}

var errAdd = errors.New("The server returned an error, which port or host may have been occupied or not allowed to open.")

func StartFromFile(path string) {
	first := true
	cnf, err := config.NewConfig(path)
	if err != nil || cnf.CommonConfig == nil {
		logs.Error("Config file %s loading error %s", path, err.Error())
		os.Exit(0)
	}
	logs.Info("Loading configuration file %s successfully", path)

	SetTlsEnable(cnf.CommonConfig.TlsEnable)
	logs.Info("the version of client is %s, the core version of client is %s,tls enable is %t", version.VERSION, version.GetVersion(), GetTlsEnable())

	for {
		if !first && !cnf.CommonConfig.AutoReconnection {
			return
		}
		if !first {
			logs.Info("Reconnecting...")
			time.Sleep(time.Second * 5)
		}
		first = false

		c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
		if err != nil {
			logs.Error(err)
			continue
		}

		var isPub bool
		binary.Read(c, binary.LittleEndian, &isPub)

		// get tmp password
		var b []byte
		vkey := cnf.CommonConfig.VKey
		if isPub {
			// send global configuration to server and get status of config setting
			if _, err := c.SendInfo(cnf.CommonConfig.Client, common.NEW_CONF); err != nil {
				logs.Error(err)
				continue
			}
			if !c.GetAddStatus() {
				logs.Error("the web_user may have been occupied!")
				continue
			}

			if b, err = c.GetShortContent(16); err != nil {
				logs.Error(err)
				continue
			}
			vkey = string(b)
		}

		if err := ioutil.WriteFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt"), []byte(vkey), 0600); err != nil {
			logs.Debug("Failed to write vkey file:", err)
			//continue
		}

		//send  task to server
		for _, v := range cnf.Tasks {
			if _, err := c.SendInfo(v, common.NEW_TASK); err != nil {
				logs.Error(err)
				continue
			}
			if !c.GetAddStatus() {
				logs.Error(errAdd, v.Ports, v.Remark)
				continue
			}
		}

		c.Close()
		if cnf.CommonConfig.Client.WebUserName == "" || cnf.CommonConfig.Client.WebPassword == "" {
			logs.Notice("web access login username:user password:%s", vkey)
		} else {
			logs.Notice("web access login username:%s password:%s", cnf.CommonConfig.Client.WebUserName, cnf.CommonConfig.Client.WebPassword)
		}

		NewRPClient(cnf.CommonConfig.Server, vkey, cnf.CommonConfig.Tp, cnf.CommonConfig.ProxyUrl, cnf, cnf.CommonConfig.DisconnectTime).Start()
	}
}

// Create a new connection with the server and verify it
func NewConn(tp string, vkey string, server string, connType string, proxyUrl string) (*conn.Conn, error) {
	var err error
	var connection net.Conn
	var sess *kcp.UDPSession

	timeout := time.Second * 10
	dialer := net.Dialer{Timeout: timeout}

	if tp == "tcp" {
		if proxyUrl != "" {
			u, er := url.Parse(proxyUrl)
			if er != nil {
				return nil, er
			}
			switch u.Scheme {
			case "socks5":
				n, er := proxy.FromURL(u, nil)
				if er != nil {
					return nil, er
				}
				connection, err = n.Dial("tcp", server)
			}
		} else {
			if GetTlsEnable() {
				//tls 流量加密
				conf := &tls.Config{InsecureSkipVerify: true}
				connection, err = tls.DialWithDialer(&dialer, "tcp", server, conf)
			} else {
				connection, err = dialer.Dial("tcp", server)
			}

			//header := &proxyproto.Header{
			//	Version:           1,
			//	Command:           proxyproto.PROXY,
			//	TransportProtocol: proxyproto.TCPv4,
			//	SourceAddr: &net.TCPAddr{
			//		IP:   net.ParseIP("10.1.1.1"),
			//		Port: 1000,
			//	},
			//	DestinationAddr: &net.TCPAddr{
			//		IP:   net.ParseIP("20.2.2.2"),
			//		Port: 2000,
			//	},
			//}
			//
			//_, err = header.WriteTo(connection)
			//_, err = io.WriteString(connection, "HELO")
		}
	} else {
		sess, err = kcp.DialWithOptions(server, nil, 10, 3)
		if err == nil {
			conn.SetUdpSession(sess)
			connection = sess
		}
	}

	if err != nil {
		return nil, err
	}

	defer connection.SetDeadline(time.Time{}) // 解除超时限制
	connection.SetDeadline(time.Now().Add(timeout))

	c := conn.NewConn(connection)
	if _, err := c.Write([]byte(common.CONN_TEST)); err != nil {
		return nil, err
	}
	if err := c.WriteLenContent([]byte(version.GetVersion())); err != nil {
		return nil, err
	}
	if err := c.WriteLenContent([]byte(version.VERSION)); err != nil {
		return nil, err
	}

	b, err := c.GetShortContent(32)
	if err != nil {
		logs.Error(err)
		return nil, err
	}
	if crypt.Md5(version.GetVersion()) != string(b) {
		//logs.Error("The client does not match the server version. The current core version of the client is", version.GetVersion())
		//return nil, err
	}

	if _, err := c.Write([]byte(common.Getverifyval(vkey))); err != nil {
		return nil, err
	}
	if s, err := c.ReadFlag(); err != nil {
		return nil, err
	} else if s == common.VERIFY_EER {
		return nil, errors.New(fmt.Sprintf("Validation key %s incorrect", vkey))
	}

	if _, err := c.Write([]byte(connType)); err != nil {
		return nil, err
	}
	c.SetAlive(tp)

	return c, nil
}

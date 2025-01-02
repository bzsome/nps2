package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
)

type CommonConfig struct {
	Server           string
	VKey             string
	Tp               string //bridgeType kcp or tcp
	AutoReconnection bool
	ProxyUrl         string
	Client           *file.Client
	DisconnectTime   int
}

type Config struct {
	content      string
	title        []string
	CommonConfig *CommonConfig
	Tasks        []*file.Tunnel
	Healths      []*file.Health
}

func NewConfig(path string) (c *Config, err error) {
	c = new(Config)
	var b []byte
	if b, err = common.ReadAllFromFile(path); err != nil {
		return
	} else {
		if c.content, err = common.ParseStr(string(b)); err != nil {
			return nil, err
		}
		if c.title, err = getAllTitle(c.content); err != nil {
			return
		}
		var nowIndex int
		var nextIndex int
		var nowContent string
		for i := 0; i < len(c.title); i++ {
			nowIndex = strings.Index(c.content, c.title[i]) + len(c.title[i])
			if i < len(c.title)-1 {
				nextIndex = strings.Index(c.content, c.title[i+1])
			} else {
				nextIndex = len(c.content)
			}
			nowContent = c.content[nowIndex:nextIndex]
			//health set
			if strings.Index(getTitleContent(c.title[i]), "health") == 0 {
				c.Healths = append(c.Healths, dealHealth(nowContent))
				continue
			}
			switch c.title[i] {
			case "[common]":
				c.CommonConfig = dealCommon(nowContent)
			default:
				if strings.Index(nowContent, "host") > -1 {

				} else {
					t := dealTunnel(nowContent)
					t.Remark = getTitleContent(c.title[i])
					c.Tasks = append(c.Tasks, t)
				}
			}
		}
	}
	return
}

func getTitleContent(s string) string {
	re, _ := regexp.Compile(`[\[\]]`)
	return re.ReplaceAllString(s, "")
}

func dealCommon(s string) *CommonConfig {
	c := &CommonConfig{}
	c.Client = file.NewClient("", true, true)
	c.Client.Cnf = new(file.Config)
	for _, v := range splitStr(s) {
		item := strings.Split(v, "=")
		if len(item) == 0 {
			continue
		} else if len(item) == 1 {
			item = append(item, "")
		}
		switch item[0] {
		case "server_addr":
			c.Server = item[1]
		case "vkey":
			c.VKey = item[1]
		case "conn_type":
			c.Tp = item[1]
		case "auto_reconnection":
			c.AutoReconnection = common.GetBoolByStr(item[1])
		case "basic_username":
			c.Client.Cnf.U = item[1]
		case "basic_password":
			c.Client.Cnf.P = item[1]
		case "web_password":
			c.Client.WebPassword = item[1]
		case "web_username":
			c.Client.WebUserName = item[1]
		case "compress":
			c.Client.Cnf.Compress = common.GetBoolByStr(item[1])
		case "crypt":
			c.Client.Cnf.Crypt = common.GetBoolByStr(item[1])
		case "proxy_url":
			c.ProxyUrl = item[1]
		case "rate_limit":
			c.Client.RateLimit = common.GetIntNoErrByStr(item[1])
		case "flow_limit":
			c.Client.Flow.FlowLimit = int64(common.GetIntNoErrByStr(item[1]))
		case "max_conn":
			c.Client.MaxConn = common.GetIntNoErrByStr(item[1])
		case "remark":
			c.Client.Remark = item[1]
		case "pprof_addr":
			common.InitPProfFromArg(item[1])
		case "disconnect_timeout":
			c.DisconnectTime = common.GetIntNoErrByStr(item[1])
		}
	}
	return c
}

func dealHealth(s string) *file.Health {
	h := &file.Health{}
	for _, v := range splitStr(s) {
		item := strings.Split(v, "=")
		if len(item) == 0 {
			continue
		} else if len(item) == 1 {
			item = append(item, "")
		}
		switch strings.TrimSpace(item[0]) {
		case "health_check_timeout":
			h.HealthCheckTimeout = common.GetIntNoErrByStr(item[1])
		case "health_check_max_failed":
			h.HealthMaxFail = common.GetIntNoErrByStr(item[1])
		case "health_check_interval":
			h.HealthCheckInterval = common.GetIntNoErrByStr(item[1])
		case "health_http_url":
			h.HttpHealthUrl = item[1]
		case "health_check_type":
			h.HealthCheckType = item[1]
		case "health_check_target":
			h.HealthCheckTarget = item[1]
		}
	}
	return h
}

func dealTunnel(s string) *file.Tunnel {
	t := &file.Tunnel{}
	t.Target = new(file.Target)
	for _, v := range splitStr(s) {
		item := strings.Split(v, "=")
		if len(item) == 0 {
			continue
		} else if len(item) == 1 {
			item = append(item, "")
		}
		switch strings.TrimSpace(item[0]) {
		case "server_port":
			t.Ports = item[1]
		case "server_ip":
			t.ServerIp = item[1]
		case "mode":
			t.Mode = item[1]
		case "target_addr":
			t.Target.TargetStr = strings.Replace(item[1], ",", "\n", -1)
		case "target_port":
			t.Target.TargetStr = item[1]
		case "target_ip":
			t.TargetAddr = item[1]
		case "password":
			t.Password = item[1]
		case "multi_account":
			t.MultiAccount = &file.MultiAccount{}
			if common.FileExists(item[1]) {
				if b, err := common.ReadAllFromFile(item[1]); err != nil {
					panic(err)
				} else {
					if content, err := common.ParseStr(string(b)); err != nil {
						panic(err)
					} else {
						t.MultiAccount.AccountMap = dealMultiUser(content)
					}
				}
			}
		}
	}
	return t

}

func dealMultiUser(s string) map[string]string {
	multiUserMap := make(map[string]string)
	for _, v := range splitStr(s) {
		item := strings.Split(v, "=")
		if len(item) == 0 {
			continue
		} else if len(item) == 1 {
			item = append(item, "")
		}
		multiUserMap[strings.TrimSpace(item[0])] = item[1]
	}
	return multiUserMap
}

func getAllTitle(content string) (arr []string, err error) {
	var re *regexp.Regexp
	re, err = regexp.Compile(`(?m)^\[[^\[\]\r\n]+\]`)
	if err != nil {
		return
	}
	arr = re.FindAllString(content, -1)
	m := make(map[string]bool)
	for _, v := range arr {
		if _, ok := m[v]; ok {
			err = errors.New(fmt.Sprintf("Item names %s are not allowed to be duplicated", v))
			return
		}
		m[v] = true
	}
	return
}

func splitStr(s string) (configDataArr []string) {
	if common.IsWindows() {
		configDataArr = strings.Split(s, "\r\n")
	}
	if len(configDataArr) < 3 {
		configDataArr = strings.Split(s, "\n")
	}
	return
}

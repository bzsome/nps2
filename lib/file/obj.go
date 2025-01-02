package file

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/rate"
	"github.com/pkg/errors"
)

type Flow struct {
	ExportFlow int64
	InletFlow  int64
	FlowLimit  int64
	sync.RWMutex
}

func (s *Flow) Add(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.InletFlow += int64(in)
	s.ExportFlow += int64(out)
}

type Config struct {
	U        string // username
	P        string // password
	Compress bool
	Crypt    bool
}

type Client struct {
	Cnf             *Config
	Id              int        //id
	VerifyKey       string     //verify key
	Addr            string     //the ip of client
	Remark          string     //remark
	Status          bool       //is allow connect
	IsConnect       bool       //is the client connect
	RateLimit       int        //rate /kb
	Flow            *Flow      //flow setting
	Rate            *rate.Rate //rate limit
	NoStore         bool       //no store to file
	NoDisplay       bool       //no display on web
	MaxConn         int        //the max connection num of client allow
	NowConn         int32      //the connection num of now
	WebUserName     string     //the username of web login
	WebPassword     string     //the password of web login
	ConfigConnAllow bool       //is allow connected by config file
	MaxTunnelNum    int
	Version         string
	BlackIpList     []string
	CreateTime      string
	LastOnlineTime  string
	sync.RWMutex
}

func NewClient(vKey string, noStore bool, noDisplay bool) *Client {
	return &Client{
		Cnf:       new(Config),
		Id:        0,
		VerifyKey: vKey,
		Addr:      "",
		Remark:    "",
		Status:    true,
		IsConnect: false,
		RateLimit: 0,
		Flow:      new(Flow),
		Rate:      nil,
		NoStore:   noStore,
		RWMutex:   sync.RWMutex{},
		NoDisplay: noDisplay,
	}
}

func (s *Client) CutConn() {
	atomic.AddInt32(&s.NowConn, 1)
}

func (s *Client) AddConn() {
	atomic.AddInt32(&s.NowConn, -1)
}

func (s *Client) GetConn() bool {
	if s.MaxConn == 0 || int(s.NowConn) < s.MaxConn {
		s.CutConn()
		return true
	}
	return false
}

func (s *Client) HasTunnel(t *Tunnel) (exist bool) {
	GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		v := value.(*Tunnel)
		if v.Client.Id == s.Id && v.Port == t.Port && t.Port != 0 {
			exist = true
			return false
		}
		return true
	})
	return
}

func (s *Client) GetTunnelNum() (num int) {
	GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		v := value.(*Tunnel)
		if v.Client.Id == s.Id {
			num++
		}
		return true
	})
	return
}

type Tunnel struct {
	Id           int
	Port         int
	ServerIp     string
	Mode         string
	Status       bool
	RunStatus    bool
	Client       *Client
	Ports        string
	Flow         *Flow
	Password     string
	Remark       string
	TargetAddr   string
	NoStore      bool
	IsHttp       bool
	LocalPath    string
	StripPre     string
	Target       *Target
	MultiAccount *MultiAccount
	Health
	sync.RWMutex
}

type Health struct {
	HealthCheckTimeout  int
	HealthMaxFail       int
	HealthCheckInterval int
	HealthNextTime      time.Time
	HealthMap           map[string]int
	HttpHealthUrl       string
	HealthRemoveArr     []string
	HealthCheckType     string
	HealthCheckTarget   string
	sync.RWMutex
}

type Host struct {
	Id           int
	Host         string //host
	HeaderChange string //header change
	HostChange   string //host change
	Location     string //url router
	Remark       string //remark
	Scheme       string //http https all
	CertFilePath string
	KeyFilePath  string
	NoStore      bool
	IsClose      bool
	AutoHttps    bool // 自动https
	Flow         *Flow
	Client       *Client
	Target       *Target //目标
	Health       `json:"-"`
	sync.RWMutex
}

type Target struct {
	nowIndex      int
	TargetStr     string
	TargetArr     []string
	LocalProxy    bool
	ProxyProtocol int // Proxy Protocol 配置：0=关闭, 1=v1, 2=v2
	sync.RWMutex
}

type MultiAccount struct {
	AccountMap map[string]string // multi account and pwd
}

func (s *Target) GetRandomTarget() (string, error) {
	// 初始化 TargetArr 并过滤空行
	if s.TargetArr == nil {
		lines := strings.Split(strings.ReplaceAll(s.TargetStr, "\r\n", "\n"), "\n")
		for _, v := range lines {
			trimmed := strings.TrimSpace(v) // 去除前后空白
			if trimmed != "" {
				s.TargetArr = append(s.TargetArr, trimmed)
			}
		}
	}

	// 确保 TargetArr 中有有效内容
	if len(s.TargetArr) == 1 {
		return s.TargetArr[0], nil
	}
	if len(s.TargetArr) == 0 {
		return "", errors.New("all inward-bending targets are offline")
	}

	// 锁定并更新索引
	s.Lock()
	defer s.Unlock()
	if s.nowIndex >= len(s.TargetArr)-1 {
		s.nowIndex = -1
	}
	s.nowIndex++
	return s.TargetArr[s.nowIndex], nil
}

type Glob struct {
	BlackIpList []string
	sync.RWMutex
}

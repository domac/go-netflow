package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrCollect = errors.New("fail to collect flow infomation")

var (
	flagSet  = flag.NewFlagSet("netflow", flag.ExitOnError)
	logLevel = flagSet.String("logLevel", "info", "log level")
	ports    = flagSet.String("ports", "8080,18080,28080", "port which collect")
)

type (

	//主流量信息
	RootNetFlow struct {
		InBytes   int64 `json:"in_Bytes"`
		OutBytes  int64 `json:"out_Bytes"`
		Timestamp int64 `json:"timestamp"`
	}

	//流量配置信息
	ConfigNetFlow struct {
		Open bool `json:"open"`
	}

	//流量采集主服务
	NetFlowServer struct {
		mux                sync.RWMutex
		once               sync.Once
		flowChan           chan *RootNetFlow
		openFlag           uint32
		collectIntervalSec int
		portsFlowCounters  []*collectInfo //0-in 1-out
	}

	collectInfo struct {
		port    int
		inFlow  int64
		outFlow int64
	}
)

func (rf *RootNetFlow) String() string {
	return fmt.Sprintf("in_bytes: %d, out_bytes: %d, timestamp: %d", rf.InBytes, rf.OutBytes, rf.Timestamp)
}

func NewNetFlowServer(portsList []int) *NetFlowServer {
	server := &NetFlowServer{
		flowChan:           make(chan *RootNetFlow, 60*60),
		openFlag:           0,
		collectIntervalSec: 1, //秒级采集
	}

	//初始化流量计数器
	var counters []*collectInfo
	for _, port := range portsList {
		cf := &collectInfo{
			port:    port,
			inFlow:  0,
			outFlow: 0,
		}
		counters = append(counters, cf)
	}
	server.portsFlowCounters = counters
	return server
}

func (server *NetFlowServer) Start() {

	LOG_INFO("start netflow")

	go server.syncConfig()
	//流量处理
	go server.handleNetflow()

	go server.timerFlowCollect()

}

//获取开关配置
func (server *NetFlowServer) getConfig() (string, error) {
	testConfig := "{\"in_bandwidth\":0,\"in_waterlevel\":100,\"out_bandwidth\":0,\"out_waterlevel\":100,\"open\":true,\"checklist\":1,\"modules\":{\"conn_sc\":{\"max_in_qps\":2000,\"max_out_qps\":2000}}}"
	return testConfig, nil
}

//同步配置操作
func (server *NetFlowServer) syncConfig() {
	dur := 30 * time.Second
	timer := time.NewTimer(dur)
	//读取redis配置,检测流量采集是否开启或关闭
	for {
		select {
		case <-timer.C:
			LOG_INFO("sync config")
			configText, err := server.getConfig()
			if err == nil {
				config := &ConfigNetFlow{}
				err = json.Unmarshal([]byte(configText), config)
				if err == nil {
					if config.Open {
						server.open()
					} else {
						server.close()
					}
				}
			}
			timer.Reset(dur)
		}
	}
}

func (server *NetFlowServer) timerFlowCollect() {

	//最小采集间隔
	if server.collectIntervalSec <= 0 {
		server.collectIntervalSec = 1
	}

	dur := time.Duration(server.collectIntervalSec) * time.Second
	ticker := time.NewTicker(dur) //这里选用计时器，因为不知道collect要多久
	for {
		select {
		case <-ticker.C:
			if !server.IsClosed() {
				in, out, err := server.flowCollect()
				if err == nil {
					server.flowChan <- &RootNetFlow{
						InBytes:   in,
						OutBytes:  out,
						Timestamp: time.Now().Unix(),
					}
				} else {
					LOG_ERROR(err)
				}
			} else {
				LOG_INFO("flow collect is closed")
			}
		}
	}
}

//流量处理
func (server *NetFlowServer) handleNetflow() {
	for {
		select {
		case flow := <-server.flowChan:
			LOG_INFO_F("receive a flow: %v", flow)
		}
	}
}

func (server *NetFlowServer) IsClosed() bool {
	return atomic.LoadUint32(&server.openFlag) == 0
}

//开启流量采集
func (server *NetFlowServer) open() {
	atomic.StoreUint32(&server.openFlag, 1)
}

//关闭流量采集
func (server *NetFlowServer) close() {
	atomic.StoreUint32(&server.openFlag, 0)
}

func (server *NetFlowServer) Shutdown() {
	LOG_INFO("shutdown netflow")
}

func main() {

	//日志初始化
	_ = flagSet.Parse(os.Args[1:])

	INIT_LOG(runtime.GOOS, *logLevel)

	var portsList []int

	for _, port := range strings.Split(*ports, ",") {
		iPort, err := strconv.Atoi(port)
		if err != nil || iPort <= 0 {
			continue
		}
		portsList = append(portsList, iPort)
	}

	server := NewNetFlowServer(portsList)

	go server.Start()
	//事件监听
	_ = OnEvent(Event_EXIT, server.Shutdown)
	WaitEvent()
	EmitEvent(Event_EXIT)
	LOG_INFO("Netflow Exit")
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//测试用的配置
var testConfig = "{\"open\":true}"

var (
	flagSet  = flag.NewFlagSet("netFlow", flag.ExitOnError)
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
		portsList          []int
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
		portsList:          portsList,
	}

	server.cleanRecords()
	server.resetFlow()
	return server
}

func (server *NetFlowServer) Start() {

	LOG_INFO("start netflow")

	go server.syncConfig()
	//流量处理
	go server.handleNetflow()

	go server.timerFlowCollect()

	go server.openApi()

}

//获取开关配置
func (server *NetFlowServer) getConfig() (string, error) {
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
				} else {
					LOG_ERROR(err)
					server.close()
				}
			}
			timer.Reset(dur)
		}
	}
}

//重置流量状态
func (server *NetFlowServer) resetFlow() {
	LOG_INFO(">>>>>>>>>>>>>>>>> reset flow status")
	//初始化流量计数器
	var counters []*collectInfo
	for _, port := range server.portsList {
		cf := &collectInfo{
			port:    port,
			inFlow:  0,
			outFlow: 0,
		}
		counters = append(counters, cf)
	}
	server.portsFlowCounters = counters
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
	if atomic.CompareAndSwapUint32(&server.openFlag, 0, 1) {
		LOG_INFO_F("turn netflow collect: Off -> On")
		server.mux.Lock()
		defer server.mux.Unlock()
		server.setupRecords()
	}
}

//关闭流量采集
func (server *NetFlowServer) close() {
	if atomic.CompareAndSwapUint32(&server.openFlag, 1, 0) {
		LOG_INFO_F("turn netflow collect: On -> Off")
		server.mux.Lock()
		defer server.mux.Unlock()
		server.cleanRecords()
		server.resetFlow()
	}
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
	_ = OnEvent(EventExit, server.Shutdown)
	WaitEvent()
	EmitEvent(EventExit)
	LOG_INFO("Netflow Exit")
}

func (server *NetFlowServer) openApi() {
	http.HandleFunc("/on", server.testOnHandler)
	http.HandleFunc("/off", server.testOffHandler)

	var err error
	err = http.ListenAndServe("0.0.0.0:25555", nil)
	if err != nil {
		LOG_ERROR(err)
		panic(err)
	}
}

func (server *NetFlowServer) testOnHandler(rspWriter http.ResponseWriter, req *http.Request) {
	LOG_INFO("-------------------> collect on")
	testConfig = "{\"open\":true}"
	_, _ = rspWriter.Write([]byte("on ok"))
}

func (server *NetFlowServer) testOffHandler(rspWriter http.ResponseWriter, req *http.Request) {
	LOG_INFO("-------------------> collect off")
	testConfig = "{\"open\":false}"
	_, _ = rspWriter.Write([]byte("off ok"))
}

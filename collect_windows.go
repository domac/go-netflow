// +build windows

package main

func (server *NetFlowServer) flowCollect() (in, out int64, err error) {
	LOG_INFO("collect in windows")
	return
}

func (server *NetFlowServer) cleanRecords() {

}

func (server *NetFlowServer) setupRecords() {

}

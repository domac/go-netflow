package main

import (
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"syscall"
)

const (
	EventExit = "exit"
)

var (
	Events = make(map[string][]func(), 2)
)

// ------------  事件管理 ---------------

//注册事件
func OnEvent(name string, fs ...func()) error {
	evs, ok := Events[name]
	if !ok {
		evs = make([]func(), 0, len(fs))
	}

	for _, f := range fs {
		fp := reflect.ValueOf(f).Pointer()
		for i := 0; i < len(evs); i++ {
			if reflect.ValueOf(evs[i]).Pointer() == fp {
				return fmt.Errorf("func[%v] already exists in event[%s]", fp, name)
			}
		}
		evs = append(evs, f)
	}
	Events[name] = evs
	return nil
}

//触发指定事件
func EmitEvent(name string) {
	evs, ok := Events[name]
	if !ok {
		return
	}

	for _, f := range evs {
		f()
	}
}

//触发所有事件
func EmitAllEvents() {
	for _, fs := range Events {
		for _, f := range fs {
			f()
		}
	}
	return
}

//事件下线
func OffEvent(name string, f func(interface{})) error {
	evs, ok := Events[name]
	if !ok || len(evs) == 0 {
		return fmt.Errorf("envet[%s] doesn't have any funcs", name)
	}

	fp := reflect.ValueOf(f).Pointer()
	for i := 0; i < len(evs); i++ {
		if reflect.ValueOf(evs[i]).Pointer() == fp {
			evs = append(evs[:i], evs[i+1:]...)
			Events[name] = evs
			return nil
		}
	}

	return fmt.Errorf("%v func dones't exist in event[%s]", fp, name)
}

//所有事件下线
func OffAllEvents(name string) error {
	Events[name] = nil
	return nil
}

//事件等待，用于阻塞
//建议服务使用这种方式，而不是笼统地使用http的serve()方式去阻塞进程
func WaitEvent(sig ...os.Signal) os.Signal {
	c := make(chan os.Signal, 1)
	if len(sig) == 0 {
		//默认事件处理
		//因为Linux版本，supervistor stop的时候发送 sigterm 信号
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	} else {
		//自定义的信号通知处理
		signal.Notify(c, sig...)
	}
	return <-c
}

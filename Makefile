.PHONY: all
# 被编译的文件
BUILDFILE=main.go
# 编译后的静态链接库文件名称
TARGETNAME=go-netflow
# GOOS为目标主机系统 
# mac os : "darwin"
# linux  : "linux"
# windows: "windows"
GOOS=linux
# GOARCH为目标主机CPU架构, 默认为amd64 
GOARCH=amd64

all: format test build

test:
	go test -v . 

format:
	gofmt -w .

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-s -w" -v -o $(TARGETNAME)
clean:
	go clean -i
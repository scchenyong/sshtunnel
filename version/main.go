package main

import (
	"encoding/json"
	"github.com/scchenyong/sshtunnel"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var sts []*sshtunnel.Config
	f, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Printf("载入配置文件出错, 错误: %v", err)
		os.Exit(-1)
	}
	err = json.Unmarshal(f, &sts)
	if nil != err {
		log.Printf("解析配置文件内容出错, 错误: %v", err)
		os.Exit(-1)
	}

	var tunnels []*sshtunnel.SSHTunnel
	for _, st := range sts {
		tunnel := sshtunnel.NewSSHTunnel(st)
		tunnel.Start()
		tunnels = append(tunnels, tunnel)
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-signalChan
	for _, t := range tunnels {
		t.Close()
	}
	os.Exit(0)
}

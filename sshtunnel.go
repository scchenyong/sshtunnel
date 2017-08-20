package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type HostConfig struct {
	Host string
	Port int32
}

type TunnelConfig struct {
	Remote HostConfig
	Local  HostConfig
}

type SSHConfig struct {
	HostConfig
	UserName string
	Password string
	Tunnels  []TunnelConfig
}

type TunnelAddr struct {
	LocalAddr  string
	RemoteAddr string
}

type TunnelConn struct {
	SSHConn *SSHConn
	TunnelAddr
	LocalListener net.Listener
	LocalConn     net.Conn
	RemoteConn    net.Conn
	Number        int
}

type SSHConn struct {
	sync.WaitGroup
	SSHConfig
	SSHClient *ssh.Client
}

var SSHConfigs []SSHConfig

func init() {
	if len(os.Args) == 1 {
		log.Println(`请输入配置文件路径参数.`)
		return
	}
	configpath := os.Args[1]
	configFile, err := ioutil.ReadFile(configpath)

	if err != nil {
		log.Printf(`载入配置文件出错: %s`, err)
		os.Exit(-1)
	}
	err = json.Unmarshal(configFile, &SSHConfigs)
	if nil != err {
		log.Printf(`解析配置文件内容出错: %s`, err)
		os.Exit(-1)
	}
}

func main() {
	var cwg sync.WaitGroup
	for _, sshConfig := range SSHConfigs {
		cwg.Add(1)
		conn := SSHConn{SSHConfig: sshConfig}
		go conn.start(cwg)
	}
	cwg.Wait()
}

func (sc *SSHConn) start(wg sync.WaitGroup) {
	defer wg.Done()

	var auth []ssh.AuthMethod
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(sc.Password))

	sshConfig := &ssh.ClientConfig{
		User: sc.UserName,
		Auth: auth,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	serverAddr := fmt.Sprintf("%s:%d", sc.Host, sc.Port)
	sshClientConn, err := ssh.Dial("tcp", serverAddr, sshConfig)
	if err != nil {
		log.Printf(`连接SSH服务器失败: %s`, err)
		return
	}
	defer sshClientConn.Close()

	sc.SSHClient = sshClientConn
	log.Printf("连接到SSH服务器成功: %s", serverAddr)

	sc.keepalive()

	var twg sync.WaitGroup
	for _, tunnelConfig := range sc.Tunnels {
		twg.Add(1)

		tunnelAddr := TunnelAddr{}
		tunnelAddr.LocalAddr = fmt.Sprintf("%s:%d", tunnelConfig.Local.Host, tunnelConfig.Local.Port)
		tunnelAddr.RemoteAddr = fmt.Sprintf("%s:%d", tunnelConfig.Remote.Host, tunnelConfig.Remote.Port)

		conn := TunnelConn{}
		conn.SSHConn = sc
		conn.TunnelAddr = tunnelAddr
		conn.Number = 0
		go conn.connect(twg)
	}
	twg.Wait()
}

func (tc *TunnelConn) connect(wg sync.WaitGroup) {
	defer wg.Done()

	fmt.Println(tc.RemoteAddr, tc.LocalAddr)

	localListener, err := net.Listen("tcp", tc.LocalAddr)
	if err != nil {
		log.Printf(`开启本地TCP端口监听失败: %s`, err)
		return
	}
	defer localListener.Close()
	tc.LocalListener = localListener
	for {
		tc.accept()
	}
}

func (tc *TunnelConn) accept() {
	localConn, err := tc.LocalListener.Accept()
	if err != nil {
		log.Printf(`接受来自本地的连接失败: %s`, err)
		return
	}
	tc.Number = tc.Number + 1
	log.Printf(`接受来自本地的连接：%s:%d`, tc.LocalAddr, tc.Number)
	tc.LocalConn = localConn
	sshConn, err := tc.SSHConn.SSHClient.Dial("tcp", tc.RemoteAddr)
	if err != nil {
		log.Printf(`连接远程服务器失败: %s`, err)
		return
	}
	tc.RemoteConn = sshConn
	go tc.transfer(tc.Number)
}

func (tc *TunnelConn) transfer(id int) {
	go func() {
		_, err := io.Copy(tc.RemoteConn, tc.LocalConn)
		if err != nil && err != io.EOF {
			log.Printf(`上传数据出错: %s`, err)
		}
		log.Printf(`断开来自本地的连接：%s:%d`, tc.LocalAddr, id)
	}()
	go func() {
		_, err := io.Copy(tc.LocalConn, tc.RemoteConn)
		if err != nil && err != io.EOF {
			log.Printf(`下载数据出错: %s`, err)
		}
		log.Printf(`断开来自本地的连接：%s:%d`, tc.LocalAddr, id)
	}()
}

func (sc *SSHConn) keepalive() {
	go func() {
		sshSession, err := sc.SSHClient.NewSession()
		if err != nil {
			log.Printf(`开启SSH会话失败: %s`, err)
			return
		}
		defer sshSession.Close()
		write, err := sshSession.StdinPipe()
		if err != nil {
			log.Println("建立SSH会话通道出错", err)
			return
		}
		defer write.Close()
		for {
			write.Write([]byte("\n"))
			time.Sleep(5 * time.Minute)
		}
	}()
}

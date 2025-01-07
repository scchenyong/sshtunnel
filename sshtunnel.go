package sshtunnel

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"log"
	"math"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	SSHConnectDefaultTimeout = 5 * time.Second
	SSHReConnectTime         = 5 * time.Second
)

type SSHTunnel struct {
	config    *Config
	sshClient *ssh.Client
	closed    bool
	closeOnce sync.Once
}

func NewSSHTunnel(config *Config) *SSHTunnel {
	st := new(SSHTunnel)
	st.config = config
	return st
}

func (t *SSHTunnel) Start() {
	if len(t.config.Pass) == 0 {
		t.setPass()
	}
	t.createTunnel()
}

func (t *SSHTunnel) Close() {
	t.closeOnce.Do(func() {
		t.closed = true
		if nil != t.sshClient {
			t.sshClient.Close()
		}
	})
}

func (t *SSHTunnel) sshSessionCheck() {
	session, err := t.sshClient.NewSession()
	if err != nil {
		t.sshClient = nil
		return
	}
	session.Close()
}

func (t *SSHTunnel) GetSSHClient() (*ssh.Client, error) {
	if t.sshClient != nil {
		return t.sshClient, nil
	}
	timeout := SSHConnectDefaultTimeout
	if t.config.Timeout > 0 {
		timeout = time.Duration(t.config.Timeout) * time.Second
	}
	sc := &ssh.ClientConfig{
		User: t.config.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(t.config.Pass),
		},
		Timeout:         timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	var err error
	t.sshClient, err = ssh.Dial("tcp", t.config.Addr, sc)
	if err != nil {
		return nil, err
	}
	log.Printf("连接到服务器成功: %s", t.config.Addr)
	return t.sshClient, err
}

func (t *SSHTunnel) createTunnel() {
	t.initSSHClient()
	for _, tunnel := range t.config.Tunnels {
		if !tunnel.IsInput {
			go t.createLocalOutput(tunnel)
			continue
		}
		go t.createRemoteInput(tunnel)
	}
}

func (t *SSHTunnel) reconnectRemote(tunnel Tunnel) {
	if t.closed {
		return
	}
	t.initSSHClient()
	t.createRemoteInput(tunnel)
}

func (t *SSHTunnel) createRemoteInput(tunnel Tunnel) {
	defer t.reconnectRemote(tunnel)
	tid := fmt.Sprintf("%s-%s", tunnel.Remote, tunnel.Local)
	log.Printf("隧道[%s]远端接收准备开启...", tid)
	sc, err := t.GetSSHClient()
	if err != nil {
		log.Printf("隧道[%s]远端服务接入失败, 错误: %v", tid, err)
		return
	}
	ll, err := sc.Listen("tcp", tunnel.Remote)
	if err != nil {
		log.Printf("隧道[%s]开启远端接收失败, 错误: %v", tid, err)
		t.sshSessionCheck()
		return
	}
	log.Printf("隧道[%s]开启远端接收成功", tid)
	defer func() {
		ll.Close()
		log.Printf("隧道[%s]远端接收关闭!", tid)
	}()
	log.Printf("隧道[%s]远端接收开启!", tid)
	cno := int64(0)
	for {
		var lc net.Conn
		lc, err = ll.Accept()
		if err != nil {
			log.Printf("隧道[%s]远端接收连接失败, 错误: %v", tid, err)
			return
		}
		if cno >= math.MaxInt64 {
			cno = 0
		}
		cno += 1
		go t.handleRemoteConnect(tid, cno, lc, tunnel.Local)
	}
}

func (t *SSHTunnel) createLocalOutput(tunnel Tunnel) {
	tid := fmt.Sprintf("%s-%s", tunnel.Local, tunnel.Remote)
	log.Printf("隧道[%s]本地接收准备开启...", tid)
	ll, err := net.Listen("tcp", tunnel.Local)
	if err != nil {
		log.Printf("隧道[%s]开启本地接收失败, 错误: %v", tid, err)
		return
	}
	log.Printf("隧道[%s]开启本地接收成功", tid)
	defer func() {
		ll.Close()
		log.Printf("隧道[%s]本地接收关闭!", tid)
	}()
	log.Printf("隧道[%s]本地接收开启!", tid)
	cno := int64(0)
	for {
		var lc net.Conn
		lc, err = ll.Accept()
		if err != nil {
			log.Printf("隧道[%s]本地接收连接失败, 错误: %v", tid, err)
			return
		}
		if cno >= math.MaxInt64 {
			cno = 0
		}
		cno += 1
		go t.handleLocalConnect(tid, cno, lc, tunnel.Remote)
	}
}

func (t *SSHTunnel) handleLocalConnect(tid string, cno int64, lc net.Conn, remote string) {
	defer lc.Close()
	sc, err := t.GetSSHClient()
	if err != nil {
		log.Printf("隧道[%s]远端服务接入失败, 错误: %v", tid, err)
		return
	}
	rc, err := sc.Dial("tcp", remote)
	if err != nil {
		log.Printf("隧道[%s]获取远端服务连接失败, 错误: %v", tid, err)
		t.sshSessionCheck()
		return
	}
	cid := fmt.Sprintf("%s:%d", tid, cno)
	go t.transfer(cid, lc, rc)
}

func (t *SSHTunnel) handleRemoteConnect(tid string, cno int64, rc net.Conn, local string) {
	defer rc.Close()
	lc, err := net.Dial("tcp", local)
	if err != nil {
		log.Printf("隧道[%s]获取本地服务连接失败, 错误: %v", tid, err)
		return
	}
	cid := fmt.Sprintf("%s:%d", tid, cno)
	go t.transfer(cid, rc, lc)
}

func (t *SSHTunnel) setPass() {
	fmt.Printf("请输入登陆密码[%s@%s]:", t.config.User, t.config.Addr)
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
	t.config.Pass = string(bytePassword)
	fmt.Println()
}

func (t *SSHTunnel) initSSHClient() {
	var err error
	for {
		if t.closed {
			return
		}
		t.sshClient, err = t.GetSSHClient()
		if nil != err {
			log.Printf("连接到服务器[%s]失败, 错误: %v", t.config.Addr, err)
			if strings.Contains(err.Error(), "unable to authenticate") {
				t.config.Pass = ""
				t.setPass()
				continue
			}
			log.Printf("稍等%.2fs后重试连接", SSHReConnectTime.Seconds())
			time.Sleep(SSHReConnectTime)
		}
		return
	}
}

func (t *SSHTunnel) transfer(cid string, lc net.Conn, rc net.Conn) {
	defer func() {
		rc.Close()
		lc.Close()
		log.Printf("隧道连接[%s]已断开!", cid)
	}()
	go func() {
		defer func() {
			rc.Close()
			lc.Close()
		}()
		io.Copy(rc, lc)
	}()
	log.Printf("隧道连接[%s]已连接!", cid)
	io.Copy(lc, rc)
}

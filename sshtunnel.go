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
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"strings"
)

type Tunnel struct {
	Remote string
	Local  string
}

type SSHTunnel struct {
	sshClient  *ssh.Client
	Addr       string
	User       string
	Pass       string
	Tunnels    []Tunnel
	BufferSise int64
	Timeout    time.Duration
}

func (st *SSHTunnel) Close() {
	if nil != st.sshClient {
		st.sshClient.Close()
	}
}

func (st *SSHTunnel) GetSSHClient() (*ssh.Client, error) {
	if st.sshClient != nil {
		return st.sshClient, nil
	}
	var auth []ssh.AuthMethod
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(st.Pass))

	sc := &ssh.ClientConfig{
		User: st.User,
		Auth: auth,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: st.Timeout * time.Millisecond,
	}
	var err error
	st.sshClient, err = ssh.Dial("tcp", st.Addr, sc)
	if err != nil {
		return nil, err
	}
	log.Printf("连接到服务器成功: %s", st.Addr)
	return st.sshClient, err
}

func (st *SSHTunnel) ClientClose() {
	if st.sshClient != nil {
		st.sshClient.Close()
		st.sshClient = nil
	}
}

func (st *SSHTunnel) connect(t Tunnel) {
	ll, err := net.Listen("tcp", t.Local)
	if err != nil {
		log.Printf(`开启本地端口监听失败: %s, %s`, t.Local, err)
		return
	}
	defer func() {
		ll.Close()
		log.Printf(`断开隧道连接：%s <=> %s`, t.Local, t.Remote)
	}()
	log.Printf(`开启隧道：%s <=> %s`, t.Local, t.Remote)
	sno := int64(0)
	for {
		lc, err := ll.Accept()
		if err != nil {
			log.Printf(`接受来自本地的连接失败: %s`, err)
			return
		}
		log.Printf(`接收到本地连接 => %s`, t.Local)
		sc, err := st.GetSSHClient()
		if err != nil {
			log.Printf(`连接到服务器失败: %s`, err)
			lc.Close()
			continue
		}
		rc, err := sc.Dial("tcp", t.Remote)
		if err != nil {
			log.Printf(`连接到远程主机失败: %s`, err)
			st.ClientClose()
			lc.Close()
			continue
		}
		log.Printf(`连接到远程主机 => %s `, t.Remote)
		sno = sno + 1
		cid := fmt.Sprintf("%s <=> %s: %d", t.Local, t.Remote, sno)
		st.transfer(cid, lc, rc)
	}
}

func main() {
	var sts []SSHTunnel
	if len(os.Args) == 1 {
		log.Println(`缺少配置文件路径参数`)
		return
	}
	p := os.Args[1]
	f, err := ioutil.ReadFile(p)
	if err != nil {
		log.Printf(`载入配置文件出错: %s`, err)
		os.Exit(-1)
	}
	err = json.Unmarshal(f, &sts)
	if nil != err {
		log.Printf(`解析配置文件内容出错: %s`, err)
		os.Exit(-1)
	}

	var wg sync.WaitGroup
	for _, st := range sts {
		st.check()
		wg.Add(1)
		go func() {
			start(st)
			wg.Done()
		}()
		log.Printf(`启动隧道配置：%s`, st.Addr)
	}
	wg.Wait()
}

func (st *SSHTunnel) setPass() {
	fmt.Printf("请输入登陆密码[%s@%s]:", st.User, st.Addr)
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
	st.Pass = string(bytePassword)
	fmt.Println()
}

func (st *SSHTunnel) check() {
	if len(st.Pass) == 0 {
		st.setPass()
	}

	if st.BufferSise == 0 {
		st.BufferSise = 5 * 1024
	}

	if st.Timeout == 0 {
		st.Timeout = 3000
	}

	st.initSSHClient()
}

func (st *SSHTunnel) initSSHClient() {
	_, err := st.GetSSHClient()
	if nil != err {
		error := err.Error()
		log.Printf(`连接主机失败: %s`, error)
		if strings.Contains(error, "unable to authenticate") {
			st.Pass = ""
			st.setPass()
			st.initSSHClient()
			return
		}
		if strings.Contains(error, "i/o timeout") {
			log.Printf(`连接到服务器超时: %s`, st.Addr)
			os.Exit(-1)
		}
	}
}

func start(st SSHTunnel) {
	defer st.Close()
	var wg sync.WaitGroup
	for _, t := range st.Tunnels {
		wg.Add(1)
		go func(tunnel Tunnel) {
			st.connect(tunnel)
			wg.Done()
		}(t)
	}
	wg.Wait()
}

func (st SSHTunnel) transfer(cid string, lc net.Conn, rc net.Conn) {
	copyBufPool := sync.Pool{
		New: func() interface{} {
			b := make([]byte, st.BufferSise)
			return &b
		},
	}
	go func() {
		defer lc.Close()
		defer rc.Close()
		log.Printf(`连接下行通道：%s`, cid)
		bufp := copyBufPool.Get().(*[]byte)
		defer copyBufPool.Put(bufp)
		io.CopyBuffer(lc, rc, *bufp)
		log.Printf(`断开下行通道：%s`, cid)
	}()
	go func() {
		defer rc.Close()
		defer lc.Close()
		log.Printf(`连接上行通道：%s`, cid)
		bufp := copyBufPool.Get().(*[]byte)
		defer copyBufPool.Put(bufp)
		io.CopyBuffer(rc, lc, *bufp)
		log.Printf(`断开上行通道：%s`, cid)
	}()
}

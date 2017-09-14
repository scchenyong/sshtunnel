package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
)

type Tunnel struct {
	Remote string
	Local  string
}

type SSHTunnel struct {
	sshClient *ssh.Client
	Addr      string
	User      string
	Pass      string
	Tunnels   []Tunnel
}

func (st *SSHTunnel) Close() {
	if nil != st.sshClient {
		st.sshClient.Close()
	}
}

func (st *SSHTunnel) Client() (*ssh.Client, error) {
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
	}
	var err error
	st.sshClient, err = ssh.Dial("tcp", st.Addr, sc)
	if err != nil {
		return nil, err
	}
	log.Printf("连接到服务器成功: %s", st.Addr)
	return st.sshClient, err
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
	for {
		lc, err := ll.Accept()
		if err != nil {
			log.Printf(`接受来自本地的连接失败: %s`, err)
			return
		}
		log.Printf(`接收到本地连接 => %s`, t.Local)
		sc, err := st.Client()
		if err != nil {
			log.Printf(`连接到服务器失败: %s`, err)
			return
		}
		rc, err := sc.Dial("tcp", t.Remote)
		if err != nil {
			log.Printf(`连接到远程主机失败: %s`, err)
			return
		}
		log.Printf(`连接到远程主机 => %s `, t.Remote)
		go transfer(lc, rc)
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
		wg.Add(1)
		go func() {
			start(st)
			wg.Done()
		}()
		log.Printf(`启动隧道配置：%s`, st.Addr)
	}
	wg.Wait()
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

func transfer(lc net.Conn, rc net.Conn) {
	go func() {
		defer lc.Close()
		defer rc.Close()
		io.Copy(rc, lc)
	}()
	go func() {
		defer rc.Close()
		defer lc.Close()
		io.Copy(lc, rc)
	}()
}

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

type Tunnel struct {
	Remote string
	Local  string
}

type SSHTunnel struct {
	Addr    string
	User    string
	Pass    string
	Tunnels []Tunnel
}

var sts []SSHTunnel

func init() {
	if len(os.Args) == 1 {
		log.Println(`请输入配置文件路径参数.`)
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
}

func main() {
	var wg sync.WaitGroup
	for _, st := range sts {
		wg.Add(1)
		go func() {
			start(st)
			wg.Done()
		}()
	}
	wg.Wait()
}

func start(st SSHTunnel) {
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
	scon, err := ssh.Dial("tcp", st.Addr, sc)
	if err != nil {
		log.Printf(`连接SSH服务器失败: %s`, err)
		return
	}
	defer scon.Close()

	log.Printf("连接到SSH服务器成功: %s", st.Addr)

	go keepalive(scon)

	var wg sync.WaitGroup
	for _, t := range st.Tunnels {
		wg.Add(1)
		go func(tunnel Tunnel) {
			connect(scon, tunnel)
			wg.Done()
		}(t)
	}
	wg.Wait()
}

func connect(scon *ssh.Client, t Tunnel) {
	fmt.Println(t.Remote, t.Local)

	ll, err := net.Listen("tcp", t.Local)
	if err != nil {
		log.Printf(`开启本地TCP端口监听失败: %s`, err)
		return
	}
	sno := 0
	for {
		lc, err := ll.Accept()
		if err != nil {
			log.Printf(`接受来自本地的连接失败: %s`, err)
			return
		}
		sno = sno + 1
		cid := fmt.Sprintf("%s <=> %s: %d", t.Local, t.Remote, sno)
		log.Printf(`接受来自本地的连接：%s `, cid)
		rc, err := scon.Dial("tcp", t.Remote)
		if err != nil {
			log.Printf(`连接远程服务器失败: %s`, err)
			return
		}
		go transfer(cid, lc, rc)
	}
}

func transfer(cid string, lc net.Conn, rc net.Conn) {
	go func() {
		io.Copy(rc, lc)
		log.Printf(`断开上行通道：%s`, cid)
		lc.Close()
		rc.Close()
	}()
	go func() {
		io.Copy(lc, rc)
		log.Printf(`断开下行通道：%s`, cid)
		rc.Close()
		lc.Close()
	}()
}

func keepalive(scon *ssh.Client) {
	ss, err := scon.NewSession()
	if err != nil {
		log.Printf(`开启SSH会话失败: %s`, err)
		return
	}
	defer ss.Close()
	w, err := ss.StdinPipe()
	if err != nil {
		log.Println("建立SSH会话通道出错", err)
		return
	}
	defer w.Close()
	for {
		w.Write([]byte("\n"))
		time.Sleep(5 * time.Minute)
	}
}

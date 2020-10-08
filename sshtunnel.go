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
	"syscall"
	"time"
)

type tunnelConnect struct {
	id int64
	net.Conn
	*SSHTunnel
}

func (t *tunnelConnect) Close() error {
	t.Conn.Close()
	t.SSHTunnel.closeConnect(t.id)
	return nil
}

type SSHTunnel struct {
	config   *Config
	client   *ssh.Client
	bufPool  *BufferPool
	connects map[int64]net.Conn
}

func NewSSHTunnel(config *Config) *SSHTunnel {
	st := new(SSHTunnel)
	st.config = config
	st.connects = make(map[int64]net.Conn)
	return st
}

func (st *SSHTunnel) Start() {
	if len(st.config.Pass) == 0 {
		st.setPass()
	}
	st.initSSHClient()
	for _, t := range st.config.Tunnels {
		go st.connect(t)
	}
}

func (st *SSHTunnel) Close() {
	if nil != st.client {
		st.client.Close()
	}
	for i, conn := range st.connects {
		conn.Close()
		delete(st.connects, i)
	}
}

func (st *SSHTunnel) GetSSHClient() (*ssh.Client, error) {
	if st.client != nil {
		return st.client, nil
	}
	var auth []ssh.AuthMethod
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(st.config.Pass))

	sc := &ssh.ClientConfig{
		User: st.config.User,
		Auth: auth,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	var err error
	st.client, err = ssh.Dial("tcp", st.config.Addr, sc)
	if err != nil {
		return nil, err
	}
	log.Printf("连接到服务器成功: %s", st.config.Addr)
	return st.client, err
}

func (st *SSHTunnel) connect(t Tunnel) {
	tid := fmt.Sprintf("%s-%s", t.Local, t.Remote)
	ll, err := net.Listen("tcp", t.Local)
	if err != nil {
		log.Printf("隧道[%s]接收开启失败, 错误: %v", tid, err)
		return
	}
	defer func() {
		ll.Close()
		log.Printf("隧道[%s]接收关闭!", tid)
	}()
	log.Printf("隧道[%s]接收开启!", tid)
	sno := int64(0)
	for {
		lc, err := ll.Accept()
		if err != nil {
			log.Printf("隧道[%s]接收连接失败, 错误: %v", tid, err)
			return
		}
		sc, err := st.GetSSHClient()
		if err != nil {
			log.Printf("隧道[%s]接入服务失败, 错误: %v", tid, err)
			lc.Close()
			continue
		}
		rc, err := sc.Dial("tcp", t.Remote)
		if err != nil {
			log.Printf("隧道[%s]接入获取连接失败, 错误: %v", err)
			sc.Close()
			lc.Close()
			continue
		}
		if sno >= math.MaxInt64 {
			sno = 0
		}
		sno += 1
		cid := fmt.Sprintf("%s:%d", tid, sno)
		go st.transfer(cid, &tunnelConnect{
			id:        sno,
			Conn:      lc,
			SSHTunnel: st,
		}, rc)
	}
}

func (st *SSHTunnel) setPass() {
	fmt.Printf("请输入登陆密码[%s@%s]:", st.config.User, st.config.Addr)
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
	st.config.Pass = string(bytePassword)
	fmt.Println()
}

func (st *SSHTunnel) initSSHClient() {
	var err error
	for {
		st.client, err = st.GetSSHClient()
		if nil != err {
			error := err.Error()
			log.Printf("连接到服务器[%s]失败, 错误: %s", st.config.Addr, error)
			if strings.Contains(error, "unable to authenticate") {
				st.config.Pass = ""
				st.setPass()
				continue
			}
			if strings.Contains(error, "i/o timeout") {
				log.Printf("连接到服务器[%s]超时!", st.config.Addr)
				time.Sleep(2 * time.Second)
				continue
			}
		}
		return
	}
}

func (st *SSHTunnel) transfer(cid string, lc net.Conn, rc net.Conn) {
	defer rc.Close()
	defer lc.Close()
	go func() {
		defer lc.Close()
		defer rc.Close()
		st.copy(rc, lc)
	}()
	log.Printf("通道[%s]已连接!", cid)
	st.copy(lc, rc)
	log.Printf("通道[%s]已断开!", cid)
}

func (st *SSHTunnel) copy(in, out io.ReadWriter) {
	if st.bufPool == nil {
		st.bufPool = NewBufferPool()
	}
	buffer := st.bufPool.Get(1024 * 32)
	defer st.bufPool.Put(buffer)
	io.CopyBuffer(in, out, buffer)
}

func (st *SSHTunnel) closeConnect(id int64) {
	delete(st.connects, id)
}

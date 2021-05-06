package network

import (
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"log"
	"net"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run() {
	tcpServer, err := net.Listen("tcp", "192.168.2.222:10810")
	if err != nil {
		log.Fatalln(err)
	}
	for {
		client, err := tcpServer.Accept()
		if err != nil {
			continue
		}
		go handleCient(client)
	}
}

func handleCient(client net.Conn) {
	if err := Socks5Auth(client); err != nil {
		fmt.Println("auth error:", err)
		client.Close()
		return
	}

	target, err := Socks5Connect(client)
	if err != nil {
		fmt.Println("connect error:", err)
		client.Close()
		return
	}
	go func() {
		//handleConn(target,client)
	}()
	Socks5Forward(client, target)


}

//VER	NMETHODS	METHODS
//1	      1	        1 to 255
//VER 本次请求的协议版本号，取固定值 0x05（表示socks 5）
//NMETHODS 客户端支持的认证方式数量，可取值 1~255
//METHODS 可用的认证方式列表
func Socks5Auth(client net.Conn) (err error) {
	buf := make([]byte, 256)
	//读取版本Ver和认证方式熟练NMETHODS
	n, err := io.ReadFull(client, buf[:2])
	if n != 2 {
		return errors.New("reading header: " + err.Error())
	}

	ver, nMethods := int(buf[0]), int(buf[1])

	if ver != 5 {
		return errors.New("Version error")
	}
	//读取Methods列表
	n, err = io.ReadFull(client, buf[:nMethods])
	if n != nMethods {
		return errors.New("reading methods:" + err.Error())
	}

	//告诉客户端版本0x05,认证方式0x00(无需认证)
	n, err = client.Write([]byte{0x05, 0x00})
	if n != 2 || err != nil {
		return errors.New("write rsp err:" + err.Error())
	}

	return nil
}

// 客户端告诉服务端要连接的目标地址,协议如下
//VER	CMD	  RSV	ATYP	DST.ADDR	DST.PORT
// 1 	 1	 X'00'   1       Variable     2
//VER 0x05，老暗号了
//CMD 连接方式，0x01=CONNECT, 0x02=BIND, 0x03=UDP ASSOCIATE
//RSV 保留字段，现在没卵用
//ATYP 地址类型，0x01=IPv4，0x03=域名，0x04=IPv6
//DST.ADDR 目标地址，细节后面讲
//DST.PORT 目标端口，2字节，网络字节序（network octec order）
func Socks5Connect(client net.Conn) (net.Conn, error) {
	buf := make([]byte, 256)
	//先读取前4个字段
	n, err := io.ReadFull(client, buf[:4])

	if n != 4 {
		return nil, errors.New("reading header:" + err.Error())
	}
	ver, cmd, _, atyp := buf[0], buf[1], buf[2], buf[3]

	if ver != 5 || cmd != 1 {
		return nil, errors.New("invalid ver/cmd")
	}
	//BIND 和 UDP ASSOCIATE 这两个 cmd 暂时不支持
	//ADDR 的格式取决于 ATYP：
	addr := ""
	switch atyp {
	case 1:
		n, err = io.ReadFull(client, buf[:4])
		if n != 4 {
			return nil, errors.New("invalid IPv4: " + err.Error())
		}
		addr = fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])
	case 3:
		n, err = io.ReadFull(client, buf[:1])
		if n != 1 {
			return nil, errors.New("invalid hostname: " + err.Error())
		}
		addrLen := int(buf[0])

		n, err = io.ReadFull(client, buf[:addrLen])
		if n != addrLen {
			return nil, errors.New("invalid hostname: " + err.Error())
		}
		addr = string(buf[:addrLen])
	case 4:
		return nil, errors.New("IPv6: no supported yet")
	default:
		return nil, errors.New("invalid atyp")
	}
	//IPv6 也不管了。
	n, err = io.ReadFull(client, buf[:2])
	if n != 2 {
		return nil, errors.New("read port: " + err.Error())
	}
	port := binary.BigEndian.Uint16(buf[:2])
	destAddrPort := fmt.Sprintf("%s:%d", addr, port)
	//destAddrPort = "192.168.2.222:10808"
	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:10808", nil, proxy.Direct)

	dest, err := dialer.Dial("tcp", destAddrPort)

	//if destAddrPort =="54.203.200.122:1234"{
	//	fmt.Println("呵呵呵呵呵")
	//	go func(d net.Conn) {
	//		handleConn(d)
	//	}(dest)
	//}
	//dest, err := net.Dial("tcp", destAddrPort)
	if err != nil {
		return nil, errors.New("dial dst: " + err.Error())
	}
	//proxy.SOCKS%
	//告诉客户端准备好了,协议如下
	//VER	REP	  RSV	   ATYP	 BND.ADDR	BND.PORT
	// 1	 1	  X'00'	    1	 Variable	  2
	//BND.ADDR/PORT 本应填入 dest.LocalAddr()，但因为基本上也没甚卵用，直接用 0 填充了：
	//ATYP = 0x01 表示 IPv4，所以需要填充 6 个 0 —— 4 for ADDR, 2 for PORT
	n, err = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	if err != nil {
		dest.Close()
		return nil, errors.New("write rsp: " + err.Error())
	}
	return dest, nil
}

//转发
func Socks5Forward(client net.Conn, target net.Conn) {
	go forward(client, target)
	go forward(target, client)
}

//转发
func forward(src, dest net.Conn) {
	defer src.Close()
	defer dest.Close()
	io.Copy(src, dest)
}

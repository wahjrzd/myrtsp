// rtsp-server
package rtsp

import (
	"fmt"
	"log"
	"net"

	"github.com/spf13/viper"
)

type RtspServer struct {
	Host     string
	Port     uint16
	listener *net.TCPListener
	bQuit    bool
	/**/
}

func NewRtspServer() *RtspServer {
	return &RtspServer{
		Host:     "",
		Port:     8554,
		listener: nil,
		bQuit:    false,
	}
}

func (r *RtspServer) GetConfig() error {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("json")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		log.Println(err)
		return err
	}
	r.Host = v.GetString("host")
	r.Port = uint16(v.GetUint32("port"))
	return nil
}

func (r *RtspServer) Start() bool {
	addrStr := fmt.Sprintf(":%d", r.Port)
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	if err != nil {
		log.Println(err)
		return false
	}
	if r.listener, err = net.ListenTCP("tcp", addr); err != nil {
		log.Println(err)
		return false
	}
	log.Println("start listen on", addr)
	for r.bQuit == false {
		conn, err := r.listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetReadBuffer(1024 * 50)
			tcpConn.SetWriteBuffer(1024 * 50)
		}
		cc := NewConnection(conn, r)
		go cc.Start()
	}
	return true
}

func (r *RtspServer) Stop() {
	r.bQuit = true
	r.listener.Close()
}

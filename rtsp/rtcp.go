// rtcp
package rtsp

import (
	"bytes"
	"fmt"
	"math/rand"
)

type RTCPPacket struct {
	SenderSSRC uint32
	length     uint16
	Identifier uint32
}

type RTCPPacketType int

const (
	RTCP_PT_SR RTCPPacketType = 200 + iota
	RTCP_PT_RR
	RTCP_PT_SDES
	RTCP_PT_BYE
	RTCP_PT_APP
)

func NewRTCP(id uint32) *RTCPPacket {
	return &RTCPPacket{
		SenderSSRC: rand.Uint32(),
		length:     0,
		Identifier: id,
	}
}

func (r *RTCPPacket) GenerateRR() {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(0x81)
	buf.WriteByte(byte(RTCP_PT_RR))

	fmt.Printf("elllad")
}

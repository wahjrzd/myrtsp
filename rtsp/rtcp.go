// rtcp
package rtsp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
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

func (r *RTCPPacket) GenerateRR(id uint32, lastTime uint32) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(0x81)                               //version(10)padding(0)rc(00001)
	buf.WriteByte(byte(RTCP_PT_RR))                   //packettype(201)
	binary.Write(buf, binary.BigEndian, uint16(7))    //length
	binary.Write(buf, binary.BigEndian, r.SenderSSRC) //sender ssrc
	/*source1*/
	binary.Write(buf, binary.BigEndian, id) /*rtp ssrc 32bits*/
	buf.WriteByte(0)                        /*fraction lost 8bits*/
	buf.WriteByte(0xff)
	buf.WriteByte(0xff)
	buf.WriteByte(0xff) /*cumulative number of packets lost 24bits*/

	//seq_num + (65536 * wrap_around_count)
	binary.Write(buf, binary.BigEndian, uint16(1))   /*rtcp.ssrc.high_cycles Sequence number cycles count 16bits*/
	binary.Write(buf, binary.BigEndian, uint16(235)) /*rtcp.ssrc.high_seq Higest sequence number received 16bits*/

	binary.Write(buf, binary.BigEndian, uint32(235)) /*rtcp.ssrc.jitter 32bits*/
	binary.Write(buf, binary.BigEndian, lastTime)    /*rtcp.ssrc.lsr 32bits*/
	//Delay since last sr TimeStamp if no sr it set zero
	binary.Write(buf, binary.BigEndian, lastTime) /*rtcp.ssrc.dlsr 32bits x(second)*65536 单位1/65536 second*/
	return buf.Bytes()
}

func (r *RTCPPacket) GenerateSD() []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(0x81)
	buf.WriteByte(byte(RTCP_PT_SDES))

	hostName, _ := os.Hostname()
	x := 1 + 1 + len(hostName) + 1
	y := (x+3)/4*4 + 4
	itemData := make([]byte, y)
	packetLength := y / 4

	binary.Write(buf, binary.BigEndian, uint16(packetLength))

	//chunk 1
	binary.BigEndian.PutUint32(itemData, r.SenderSSRC)
	itemData[4] = 0x01                   //rtcp.sdes.type
	itemData[5] = byte(len(hostName))    //length
	copy(itemData[6:], []byte(hostName)) //text
	buf.Write(itemData)
	return buf.Bytes()
}

// client-connection
package rtsp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

const allowedCommandNames = "OPTIONS, DESCRIBE, SETUP, TEARDOWN, PLAY"

type ClientConnection struct {
	Conn        net.Conn
	rtsp        *RtspServer
	ConnRW      *bufio.ReadWriter
	RtpChannel  int
	RtcpChannel int
	ID          string
}

func NewConnection(con net.Conn, r *RtspServer) *ClientConnection {
	return &ClientConnection{
		Conn:        con,
		rtsp:        r,
		ConnRW:      bufio.NewReadWriter(bufio.NewReaderSize(con, 204800), bufio.NewWriterSize(con, 204800)),
		RtpChannel:  -1,
		RtcpChannel: -1,
		ID:          "",
	}
}

func (c *ClientConnection) Start() {
	defer c.Conn.Close()
	log.Printf("new connect:%v\n", c.Conn.RemoteAddr())
	buf1 := make([]byte, 1)
	buf2 := make([]byte, 2)
	for {
		if _, err := io.ReadFull(c.ConnRW, buf1); err != nil {
			log.Println(err)
			return
		}
		if buf1[0] == 0x24 {
			if _, err := io.ReadFull(c.ConnRW, buf1); err != nil {
				log.Println(err)
				return
			} /*channel*/
			if _, err := io.ReadFull(c.ConnRW, buf2); err != nil {
				log.Println(err)
				return
			} /*size*/
			dataSize := binary.BigEndian.Uint16(buf2)
			data := make([]byte, dataSize)
			if _, err := io.ReadFull(c.ConnRW, data); err != nil {
				log.Println(err)
				return
			}
			if int(buf1[0]) == c.RtcpChannel {
				//rc := data[0] & 0x1f
				switch data[1] {
				case 200: /*sender report*/
					log.Printf("rtcp packet type sender report.dataSize:%d\n", dataSize)
				case 201: /*receiver report*/
					log.Printf("rtcp packet type receiver report.dataSize:%d\n", dataSize)
				case 202: /*source description item*/
					log.Printf("rtcp packet type source description item.dataSize:%d\n", dataSize)
				case 203: /*byte*/
					log.Printf("rtcp packet type byte.dataSize:%d\n", dataSize)
				case 204: /*app*/
					log.Printf("rtcp packet type app.dataSize:%d\n", dataSize)
				}
			} else if int(buf1[0]) == c.RtpChannel {

			}

		} else {
			reqBuf := bytes.NewBuffer(nil)
			reqBuf.Write(buf1)
			for {
				if line, isPrefix, err := c.ConnRW.ReadLine(); err != nil {
					log.Println(err)
					return
				} else {
					reqBuf.Write(line)
					if !isPrefix {
						reqBuf.WriteString("\r\n")
					}
					if len(line) == 0 {
						log.Println(reqBuf.String())
						req := ParseReqBuf(reqBuf.String())
						if req == nil {
							break
						}

						var resp string
						switch req.Method {
						case "OPTIONS":
							resp = c.handleCmdOPTIONS(req.Headers["CSeq"])
						case "DESCRIBE":
							resp = c.handleCmdDESCRIBE(req.Headers["CSeq"])
						case "SETUP":
							ts := req.Headers["Transport"] /*Transport: RTP/AVP/TCP;unicast;interleaved=0-1*/
							mtcp := regexp.MustCompile("interleaved=(\\d+)-(\\d+)?")
							mudp := regexp.MustCompile("client_port=(\\d+)-(\\d+)?")
							if mts := mtcp.FindStringSubmatch(ts); mts != nil {
								c.RtpChannel, err = strconv.Atoi(mts[1])
								c.RtcpChannel, err = strconv.Atoi(mts[2])
								c.ID = fmt.Sprintf("%X", unsafe.Pointer(c))
								resp = c.handleCmdSETUP(req.Headers["CSeq"], 0)
							} else if mus := mudp.FindStringSubmatch(ts); mus != nil {
								log.Println("udp")
							}
						case "PLAY":
							resp = c.handleCmdPLAY(req.Headers["CSeq"])
						case "TEARDOWN":
							resp = c.handleCmdTEARDOWN(req.Headers["CSeq"])
						default:
							resp = c.handleCmdNOTFOUND(req.Headers["CSeq"])
						}
						log.Println(resp)
						c.ConnRW.WriteString(resp)
						c.ConnRW.Flush()
						if strings.Compare(req.Method, "PLAY") == 0 {
							log.Println("start play")
							go c.StartPlay()
						}
						break
					}
				}
			}
		}
	}
}

func (c *ClientConnection) handleCmdOPTIONS(cseq string) string {
	return fmt.Sprintf("RTSP/1.0 200 OK\r\nCSeq: %s\r\nDate: %s\r\nPublic: %s\r\n\r\n", cseq,
		time.Now().Format("Mon, Jan 2 2006 15:04:05 GMT"), allowedCommandNames)
}

func (c *ClientConnection) handleCmdDESCRIBE(cseq string) string {
	sdp := fmt.Sprintf("v=0\r\n"+
		"o=- %d %d IN IP4 %s\r\n"+
		"c=IN IP4 %s\r\n"+
		"t=0 0\r\n"+
		"a=range:npt=0-\r\n"+
		"m=video 0 RTP/AVP 96\r\n"+
		"a=rtpmap:96 H264/90000\r\n", time.Now().Unix(), time.Now().Unix(), c.rtsp.Host, c.rtsp.Host)

	return fmt.Sprintf("RTSP/1.0 200 OK\r\nCSeq: %s\r\nDate: %s\r\nContent-Type: application/sdp\r\nContent-Length: %d\r\n\r\n%s", cseq,
		time.Now().Format("Mon, Jan 2 2006 15:04:05 GMT"), len(sdp), sdp)
}

func (c *ClientConnection) handleCmdSETUP(cseq string, streamMode int) string {
	if streamMode == 0 { /*tcp*/
		return fmt.Sprintf("RTSP/1.0 200 OK\r\nCSeq: %s\r\nDate: %s\r\nTransport: RTP/AVP/TCP;unicast;interleaved=%d-%d;\r\nSession: %s\r\n\r\n", cseq,
			time.Now().Format("Mon, Jan 2 2006 15:04:05 GMT"), c.RtpChannel, c.RtcpChannel, c.ID)
	} else if streamMode == 1 { /*udp*/
		return fmt.Sprintf("RTSP/1.0 200 OK\r\nCSeq: %s\r\nDate: %s\r\nTransport: RTP/AVP;unicast;destination=%s;source=%s;client_port=%d-%d;server_port=%d-%d\r\nSession: %s\r\n\r\n", cseq,
			time.Now().Format("Mon, Jan 2 2006 15:04:05 GMT"), c.RtpChannel, c.RtcpChannel, c.ID)
	}
	return ""
}

func (c *ClientConnection) handleCmdPLAY(cseq string) string {
	rng := "Range: npt=0.000-\r\n"
	rtpInfo := "RTP-Info: seq=0;rtptime=0\r\n"
	return fmt.Sprintf("RTSP/1.0 200 OK\r\nCSeq: %s\r\nDate: %s\r\n%sSession: %s\r\n%s\r\n", cseq,
		time.Now().Format("Mon, Jan 2 2006 15:04:05 GMT"), rng, c.ID, rtpInfo)
}

func (c *ClientConnection) handleCmdTEARDOWN(cseq string) string {
	return fmt.Sprintf("RTSP/1.0 200 OK\r\nCSeq: %s\r\nDate: %s\r\nSession: %s\r\nConnection: Close\r\n\r\n", cseq,
		time.Now().Format("Mon, Jan 2 2006 15:04:05 GMT"), c.ID)
}

func (c *ClientConnection) handleCmdNOTFOUND(cseq string) string {
	return fmt.Sprintf("RTSP/1.0 404 Stream Not Found\r\nCSeq: %s\r\nDate: %s\r\n", cseq, time.Now().Format("Mon, Jan 2 2006 15:04:05 GMT"))
}

func (c *ClientConnection) StartPlay() {
	mf := NewMediaFileSource()
	if err := mf.ReadFileData("2m.h264"); err != nil {
		log.Println(err)
		return
	}
	rtp := NewRtpPacket(0, 0, 0x60)
	var pts uint32 = 0
	rtpPrefix := make([]byte, 4)
	for {
		nalu := mf.GetNextNalu()
		if nalu == nil {
			log.Println("end file")
			return
		}
		var pkts []string
		if (nalu[0]&0x1f) == 5 || (nalu[0]&0x1f) == 1 {
			pkts = rtp.BuildRTPWithAVCNALU(true, nalu, pts)
		} else {
			pkts = rtp.BuildRTPWithAVCNALU(false, nalu, pts)
		}

		for _, v := range pkts {
			rtpPrefix[0] = 0x24
			rtpPrefix[1] = byte(c.RtpChannel)

			binary.BigEndian.PutUint16(rtpPrefix[2:], uint16(len(v)))
			c.ConnRW.Write(rtpPrefix)
			if _, err := c.ConnRW.WriteString(v); err != nil {
				fmt.Println(err)
				return
			} else {
				c.ConnRW.Flush()
			}
		}
		if (nalu[0]&0x1f) != 6 && (nalu[0]&0x1f) != 7 && (nalu[0]&0x1f) != 8 {
			pts += 40
			time.Sleep(time.Millisecond * 30)
		}

	}
}

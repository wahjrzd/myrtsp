// rtsp-client
package rtsp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gortc.io/sdp"
)

type ProtocolName int

const (
	TCP ProtocolName = iota
	UDP
)

type RtspClient struct {
	CSeq             uint32
	BaseUrl          string
	Host             string
	Port             uint16
	vRTPChannel      int
	vRTCPChannel     int
	aRTPChannel      int
	aRTCPChannel     int
	UserAgent        string
	Conn             net.Conn
	ConnRW           *bufio.ReadWriter
	auth             *DigestAuth
	Protocol         ProtocolName
	SessionId        string
	Scale            float32
	Speed            float32
	StartTime        float32
	EndTime          float32
	CurrentCmd       string
	VideoControlPath string
	AudioControlPath string
	rtp              *RTPunpacket
	RTPDataCallback  func([]byte, interface{})
	RTPDataUser      interface{}
	HasVideo         bool
	SendVideoSteup   bool
	HasAudio         bool
	SendAudioSetup   bool
	HasQuit          chan int
}

func NewRtspClient(rawUrl string, id string) *RtspClient {
	u, e := url.Parse(rawUrl)
	if e != nil {
		return nil
	}
	auth := NewAuth()
	auth.UserName = u.User.Username()
	pwd, ok := u.User.Password()
	if ok {
		auth.Password = pwd
	} else {
		log.Println("url not contain password")
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		port = 554
	}
	return &RtspClient{
		CSeq:            1,
		BaseUrl:         rawUrl,
		auth:            auth,
		Host:            u.Hostname(),
		Port:            uint16(port),
		vRTPChannel:     0,
		vRTCPChannel:    1,
		aRTPChannel:     2,
		aRTCPChannel:    3,
		UserAgent:       "User-Agent: Simple RTSP Client\r\n",
		Protocol:        TCP,
		Scale:           1.0,
		Speed:           1.0,
		StartTime:       0.0,
		EndTime:         -1.0,
		CurrentCmd:      "DESCRIBE",
		rtp:             NewRTPUnpacket(),
		RTPDataCallback: nil,
		HasVideo:        false,
		SendVideoSteup:  false,
		HasAudio:        false,
		SendAudioSetup:  false,
		HasQuit:         make(chan int),
	}
}

func (cli *RtspClient) OpenStream() int {
	defer cli.SetQuit()
	addr := fmt.Sprintf("%s:%d", cli.Host, cli.Port)
	conn, err := net.DialTimeout("tcp", addr, time.Second*3)
	if err != nil {
		log.Println(err)
		return 1
	}
	cli.Conn = conn
	cli.ConnRW = bufio.NewReadWriter(bufio.NewReaderSize(conn, 204800), bufio.NewWriterSize(conn, 204800))
	cli.sendRequest(cli.CurrentCmd, cli.BaseUrl, 0, 1)
	buf1 := make([]byte, 1)
	buf2 := make([]byte, 2)
	for {
		if _, err := io.ReadFull(cli.ConnRW, buf1); err != nil {
			log.Println(err)
			return 1
		}
		if buf1[0] == 0x24 {
			if _, err := io.ReadFull(cli.ConnRW, buf1); err != nil {
				log.Println(err)
				return 1
			} /*channel*/
			if _, err := io.ReadFull(cli.ConnRW, buf2); err != nil {
				log.Println(err)
				return 1
			} /*size*/

			dataSize := binary.BigEndian.Uint16(buf2)
			data := make([]byte, dataSize)
			if _, err := io.ReadFull(cli.ConnRW, data); err != nil {
				log.Println(err)
				return 1
			}

			if int(buf1[0]) == cli.vRTCPChannel {

			} else if int(buf1[0]) == cli.vRTPChannel {
				cli.rtp.InputRTPData(data, "video")
			} else if int(buf1[0]) == cli.aRTPChannel {
				//cli.rtp.InputRTPData(data, "audio")
			} else if int(buf1[0]) == cli.aRTCPChannel {

			}
		} else {
			buf := bytes.NewBuffer(nil)
			buf.Write(buf1)
			for {
				if line, isPrefix, err := cli.ConnRW.ReadLine(); err != nil {
					fmt.Println(err)
					break
				} else {
					buf.Write(line)
					if !isPrefix {
						buf.WriteString("\r\n")
					}
					if len(line) == 0 {
						log.Printf(buf.String())
						resp := parseRespBuf(buf.String())
						if resp.ResponseCode == 401 { /*need auth*/
							if authStr, ok := resp.Headers["WWW-Authenticate"]; ok {
								// if strings.HasSuffix("Digest") {

								// } else if strings.HasSuffix("Basic") {

								// }
								realm, nonce := parseWWWAuth(authStr)
								cli.auth.Realm = realm
								cli.auth.Nonce = nonce
								cli.sendRequest(cli.CurrentCmd, cli.BaseUrl, 0, 1)
							} else {
								log.Println("return 401 but not www-auth info")
								return 1
							}
						} else if resp.ResponseCode == 200 {
							if contentLengthStr, ok := resp.Headers["Content-Length"]; ok {
								contentLength, _ := strconv.Atoi(contentLengthStr)
								content := make([]byte, contentLength)
								if _, err := io.ReadFull(cli.ConnRW, content); err != nil {
									log.Println(err)
									return 1
								}
								log.Println(string(content))
								/*parse sdp*/
								var sdpSession sdp.Session
								sdpSession, err := sdp.DecodeSession(content, sdpSession)
								if err != nil {
									log.Println(err)
									return 1
								}
								d := sdp.NewDecoder(sdpSession)
								sdpMsg := &sdp.Message{}
								if err := d.Decode(sdpMsg); err != nil {
									fmt.Println(err)
									return 1
								}

								for _, v := range sdpMsg.Medias {
									switch v.Description.Type {
									case "video":
										cli.VideoControlPath = v.Attribute("control")
										rtpmap := v.Attribute("rtpmap") //96 H264/90000
										//payloadType, _ := strconv.Atoi(strings.Split(rtpmap, " ")[0])
										cli.HasVideo = true

										if strings.Contains(rtpmap, "H265") {
											cli.rtp.SetVideoCodecType("H265")
										} else if strings.Contains(rtpmap, "H264") {
											cli.rtp.SetVideoCodecType("H264")
										} else {
											log.Println(rtpmap)
										}
										//
									case "audio":
										cli.AudioControlPath = v.Attribute("control")
										_ = v.Attribute("rtpmap") //0 PCMU/8000
										cli.HasAudio = true
									}
								}
							}
							if contentBase, ok := resp.Headers["Content-Base"]; ok {
								cli.BaseUrl = contentBase
							}
							if sid, ok := resp.Headers["Session"]; ok {
								sitems := strings.Split(strings.TrimSpace(sid), ";")
								cli.SessionId = strings.TrimSpace(sitems[0])
							}
							if cli.CurrentCmd == "DESCRIBE" {
								cli.CurrentCmd = "SETUP"
								tempUri := cli.BaseUrl
								if cli.VideoControlPath != "" {
									if strings.HasSuffix(tempUri, "/") == false {
										tempUri += "/"
									}
									if strings.HasPrefix(cli.VideoControlPath, "rtsp://") {
										tempUri = cli.VideoControlPath
									} else {
										tempUri += cli.VideoControlPath
									}
								}
								cli.sendRequest(cli.CurrentCmd, tempUri, cli.vRTPChannel, cli.vRTCPChannel)
								cli.SendVideoSteup = true
							} else if cli.CurrentCmd == "SETUP" {
								if cli.HasAudio && cli.SendAudioSetup == false {
									tempUri := cli.BaseUrl
									if cli.AudioControlPath != "" {
										if strings.HasSuffix(tempUri, "/") == false {
											tempUri += "/"
										}
										if strings.HasPrefix(cli.AudioControlPath, "rtsp://") {
											tempUri = cli.AudioControlPath
										} else {
											tempUri += cli.AudioControlPath
										}
									}
									cli.sendRequest(cli.CurrentCmd, tempUri, cli.aRTPChannel, cli.aRTCPChannel)
									cli.SendAudioSetup = true
								} else {
									cli.CurrentCmd = "PLAY"
									cli.sendRequest(cli.CurrentCmd, cli.BaseUrl, 0, 1)
								}

							}
						} else {
							/*TODO*/
						}
						break
					}
				}
			}

			if cli.CurrentCmd == "TEARDOWN" {
				break
			}
		}
	}
	return 0
}

func (cli *RtspClient) sendRequest(cmd string, url string, a int, b int) {
	cli.CSeq++
	extraHeaders := bytes.NewBuffer(nil)
	authenticatorStr := cli.auth.CreateAuthenticatorString(cmd, cli.BaseUrl)
	var reqHeader string = fmt.Sprintf("%s %s RTSP/1.0\r\n"+
		"CSeq: %d\r\n"+
		"%s"+
		"%s", cmd, url, cli.CSeq, authenticatorStr, cli.UserAgent)

	extraHeaders.WriteString(reqHeader)

	if cmd == "DESCRIBE" {
		extraHeaders.WriteString("Accept: application/sdp\r\n")
	} else if cmd == "SETUP" {
		if cli.Protocol == TCP {
			transport := fmt.Sprintf("Transport: RTP/AVP/TCP;unicast;interleaved=%d-%d\r\n", a, b)
			extraHeaders.WriteString(transport)
			if cli.SessionId != "" {
				sessionStr := fmt.Sprintf("Session: %s\r\n", cli.SessionId)
				extraHeaders.WriteString(sessionStr)
			}
		} else {
			if true { /*多播*/
				//_ := fmt.Sprintf("Transport: RAW/RAW/;multicast;port=%d-%d\r\n", cli.vRTPChannel, cli.vRTCPChannel)
			} else {
				//_ := fmt.Sprintf("Transport: RAW/RAW/;unicast;client_ports=%d-%d\r\n", cli.vRTPChannel, cli.vRTCPChannel)
			}

			//extraHeaders.WriteString(transport)
		}
	} else {
		if cmd == "PLAY" {
			if cli.SessionId != "" {
				sessionStr := fmt.Sprintf("Session: %s\r\n", cli.SessionId)
				extraHeaders.WriteString(sessionStr)
			}
			if cli.Scale != 1.0 {
				scaleStr := fmt.Sprintf("Scale: %f\r\n", cli.Scale)
				extraHeaders.WriteString(scaleStr)
			}
			if cli.Speed != 1.0 {
				speedStr := fmt.Sprintf("Speed: %.3f\r\n", cli.Speed)
				extraHeaders.WriteString(speedStr)
			}
			var rangeStr string
			if cli.StartTime < 0 {
				// We're resuming from a PAUSE; there's no "Range:" header at all
			} else if cli.EndTime < 0 {
				rangeStr = fmt.Sprintf("Range: npt=%.3f-\r\n", cli.StartTime)
			} else {
				rangeStr = fmt.Sprintf("Range: npt=%.3f-%.3f\r\n", cli.StartTime, cli.EndTime)
			}
			extraHeaders.WriteString(rangeStr)
		} else {
			sessionStr := fmt.Sprintf("Session: %s\r\n", cli.SessionId)
			extraHeaders.WriteString(sessionStr)
		}
	}
	extraHeaders.WriteString("\r\n")
	log.Printf(extraHeaders.String())
	cli.ConnRW.Write(extraHeaders.Bytes())
	cli.ConnRW.Flush()
}

func (cli *RtspClient) StopStream() {
	cli.CurrentCmd = "TEARDOWN"
	cli.sendRequest(cli.CurrentCmd, cli.BaseUrl, 0, 1)
	_ = <-cli.HasQuit

	cli.Conn.Close()
}

func (cli *RtspClient) SetRawDataCallback(cb FrameCallback, arg interface{}) {
	cli.rtp.SetCallback(cb, arg)
}

func (cli *RtspClient) SetRTPDataCallback(cb func([]byte, interface{}), arg interface{}) {
	cli.RTPDataCallback = cb
	cli.RTPDataUser = arg
}

func (cli *RtspClient) SetQuit() {
	cli.HasQuit <- 1
}

// rtp-packet
package rtsp

import (
	"bytes"
	"encoding/binary"
)

const MTU = 1440

type RtpPacket struct {
	seq         uint16
	ssrc        uint32
	payloadType byte
	rtpHead     [12]byte
	buf         *bytes.Buffer
}

func NewRtpPacket(seq uint16, ssrc uint32, pt byte) *RtpPacket {
	return &RtpPacket{
		seq:         seq,
		ssrc:        ssrc,
		payloadType: pt,
		buf:         bytes.NewBuffer(nil),
	}
}

func (r *RtpPacket) buildRtpHead(mark bool, timestamp uint32) {
	r.rtpHead[0] = 0x80
	if mark {
		r.rtpHead[1] = 1 << 7
	} else {
		r.rtpHead[1] = 0
	}

	r.rtpHead[1] |= r.payloadType /*M_PT*/

	binary.BigEndian.PutUint16(r.rtpHead[2:], r.seq)
	r.seq += 1
	binary.BigEndian.PutUint32(r.rtpHead[4:], timestamp)
	binary.BigEndian.PutUint32(r.rtpHead[8:], r.ssrc)
}

func (r *RtpPacket) BuildRTPWithAVCNALU(mark bool, payload []byte, pts uint32) []string {
	var ss []string

	if len(payload) <= MTU {
		r.buildRtpHead(mark, pts*90)
		temp := make([]byte, 12+len(payload))
		copy(temp, r.rtpHead[:])
		copy(temp[12:], payload)
		ss = append(ss, string(temp))
	} else {
		k := 0
		tp := payload[0]
		i := (len(payload) - 1) / (MTU - 2)
		j := (len(payload) - 1) % (MTU - 2)
		if j != 0 {
			i += 1
		}

		for k < i {
			var s string
			if k == 0 {
				s = r.buildOneRTPWithFUA(tp, payload[1:(MTU-2)+1], false, pts, true, false)
			} else if k+1 == i {
				s = r.buildOneRTPWithFUA(tp, payload[1+k*(MTU-2):], mark, pts, false, true)
			} else {
				s = r.buildOneRTPWithFUA(tp, payload[1+k*(MTU-2):1+(k+1)*(MTU-2)], false, pts, false, false)
			}
			k++
			if len(s) != 0 {
				ss = append(ss, s)
			}
		}
	}
	return ss
}

func (r *RtpPacket) buildOneRTPWithFUA(t byte, data []byte, mark bool, pts uint32, payload_start bool, payload_end bool) string {
	if len(data) == 0 {
		return ""
	}
	r.buildRtpHead(mark, pts*90)
	//	--	FU indicator	--
	//	--	+---------------+	--
	//	--	|0|1|2|3|4|5|6|7|	--
	//	--	+-+-+-+-+-+-+-+-+	--
	//	--	|F|NRI|  Type(FUA)  |	--
	//	--	+---------------+	--
	fu_indicator := ((0xE0 & t) | 0x1c)
	//	--	FU header	--
	//	--	+---------------+	--
	//	--	|0|1|2|3|4|5|6|7|	--
	//	--	+-+-+-+-+-+-+-+-+	--
	//	--	|S|E|R|  Type(Frame)|	--
	//	--	+---------------+	--
	fu_header := 0x1F & t
	if payload_start {
		fu_header |= 0x80
	} else if payload_end {
		fu_header |= 0x40
	}
	r.buf.Reset()
	r.buf.Write(r.rtpHead[:])
	r.buf.WriteByte(fu_indicator)
	r.buf.WriteByte(fu_header)
	r.buf.Write(data)

	return r.buf.String()
}

func (r *RtpPacket) BuildRTPWithHEVCNALU(payload []byte, pts uint32) []string {
	var ss []string

	if len(payload) <= MTU {
		r.buildRtpHead(true, pts*90)
		temp := make([]byte, 12+len(payload))
		copy(temp, r.rtpHead[:])
		copy(temp[12:], payload)
		ss = append(ss, string(temp))
	} else {
		k := 0
		var head [2]byte
		head[0] = payload[0]
		head[1] = payload[1]

		i := (len(payload) - 2) / (MTU - 3)
		j := (len(payload) - 2) / (MTU - 3)
		if j != 0 {
			i += 1
		}

		for k < i {
			var s string

			if k == 0 {
				s = r.buildOneHEVCRTPWithFUA(head, payload[2:(MTU-3)+2], false, pts, true, false)
			} else if k+1 == i {
				s = r.buildOneHEVCRTPWithFUA(head, payload[2+k*(MTU-3):], true, pts, false, true)
			} else {
				s = r.buildOneHEVCRTPWithFUA(head, payload[2+k*(MTU-3):2+(k+1)*(MTU-3)], false, pts, false, false)
			}
			k++
			if len(s) != 0 {
				ss = append(ss, s)
			}
		}
	}
	return ss
}

func (r *RtpPacket) buildOneHEVCRTPWithFUA(naluHead [2]byte, data []byte, mark bool, pts uint32, payload_start bool, payload_end bool) string {
	if len(data) == 0 {
		return ""
	}
	r.buildRtpHead(mark, pts*90)

	var payloadHeader1, payloadHeader2, fuHeader byte
	payloadHeader1 = (naluHead[0] & 0x81) | (49 << 1)
	payloadHeader2 = naluHead[1]

	//	--	FU header	--
	//	--	+---------------+	--
	//	--	|0|1|2|3|4|5|6|7|	--
	//	--	+-+-+-+-+-+-+-+-+	--
	//	--	|S|E|  nalutype   |	--
	//	--	+---------------+	--
	fuHeader = (naluHead[0] & 0x7E) >> 1
	if payload_start {
		fuHeader |= 0x80
	} else if payload_end {
		fuHeader |= 0x40
	}

	r.buf.Reset()
	r.buf.Write(r.rtpHead[:])

	r.buf.WriteByte(payloadHeader1)
	r.buf.WriteByte(payloadHeader2)
	r.buf.WriteByte(fuHeader)
	r.buf.Write(data)

	return r.buf.String()
}

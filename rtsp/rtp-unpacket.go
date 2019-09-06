// rtp-unpacket
package rtsp

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type HEVCNALUnitType int

const (
	HEVC_NAL_IDR_W_RADL HEVCNALUnitType = 19 + iota
	HEVC_NAL_VPS                        = 32
	HEVC_NAL_SPS                        = 33
	HEVC_NAL_PPS                        = 34
	HEVC_NAL_AUD                        = 35
	HEVC_NAL_FD_NUT                     = 38
	HEVC_NAL_SEI_PREFIX                 = 39
	HEVC_NAL_SEI_SUFFIX                 = 40
)

type RTPunpacket struct {
	PayloadType   byte
	CodecType     string
	frameBuffer   *bytes.Buffer
	NALUHeader    [2]byte
	FrameCallback func([]byte, uint32, byte)
}

func NewRTPUnpacket(pt byte, codec string, cb func([]byte, uint32, byte)) *RTPunpacket {
	return &RTPunpacket{
		PayloadType:   pt,
		CodecType:     codec,
		frameBuffer:   bytes.NewBuffer(nil),
		FrameCallback: cb,
	}
}

func (r *RTPunpacket) InputRTPData(data []byte) {
	if data == nil || len(data) <= 12 {
		fmt.Println("rtp is nil or rtp is to small")
		return
	}

	ex := (data[0] & 0x10) >> 4
	if ex != 0 {
		fmt.Println("has extension field")
		return
	}
	mark := (data[1] & 0x80) >> 7
	// pt := data[1] & 0x7f
	// seq := binary.BigEndian.Uint16(data[2:])
	tm := binary.BigEndian.Uint32(data[4:])
	// ssrc := binary.BigEndian.Uint32(data[8:])

	//fmt.Printf("Mark:%d,PT:%d,Seq:%d,Time:%d,SSCR:%d\n", mark, pt, seq, tm, ssrc)
	var t byte = 0
	if r.CodecType == "H264" {
		t = r.parseAVCRTP(data[12:])
	} else if r.CodecType == "H265" {
		r.parseHEVCRTP(data[12:])
	} else {
		fmt.Println("unknown codecType")
	}

	if mark == 1 {
		if r.FrameCallback != nil {
			r.FrameCallback(r.frameBuffer.Bytes(), tm, t)
		}
		r.frameBuffer.Reset()
	}
}

func (r *RTPunpacket) parseHEVCRTP(data []byte) {
	if (data[0]>>1)&49 == 49 { /*fu*/
		se := data[2] >> 6
		naluType := data[2] & 0x3f
		if se == 2 { /*s bit*/
			r.NALUHeader[1] = data[1]
			r.NALUHeader[0] = (data[0] & 0x81) | (naluType << 1)
			//fmt.Printf("NaluHeader:0x%x 0x%x\n", r.NALUHeader[0], r.NALUHeader[1])
			r.frameBuffer.Write(NAL4[:])
			r.frameBuffer.Write(r.NALUHeader[:])
			r.frameBuffer.Write(data[3:])
		} else if se != 3 { /*e middle bit*/
			r.frameBuffer.Write(data[3:])
		} else {
			fmt.Println("error format packet")
		}
	} else {
		naluType := (data[0] & 0x7E) >> 1
		fmt.Printf("naluType:%d\n", naluType)
		fmt.Printf("NaluHeader:0x%x 0x%x\n", data[0], data[1])

		r.frameBuffer.Write(NAL4[:])
		r.frameBuffer.Write(data)
	}
}

func (r *RTPunpacket) parseAVCRTP(data []byte) byte {
	if (data[0] & 0x1c) == 0x1c { /*FUA*/
		se := data[1] >> 6
		if se == 2 { /*s bit*/
			r.NALUHeader[0] = (data[0] & 0xE0) | (data[1] & 0x1F)

			r.frameBuffer.Write(NAL4[:])
			r.frameBuffer.WriteByte(r.NALUHeader[0])
			r.frameBuffer.Write(data[2:])
		} else if se != 3 { /*e or 0 bit*/
			r.frameBuffer.Write(data[2:])
		} else {
			fmt.Println("avc rtp packet error")
		}
		return data[1] & 0x1F
	} else {
		r.frameBuffer.Write(NAL4[:])
		r.frameBuffer.Write(data)
		return data[0] & 0x1F
	}
}

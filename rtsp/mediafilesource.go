// mediafilesource
package rtsp

import (
	"bytes"
	"io/ioutil"
)

type MediaFileSource struct {
	Offset   uint32
	FileSize uint32
	data     []byte
}

func NewMediaFileSource() *MediaFileSource {
	return &MediaFileSource{}
}

func (m *MediaFileSource) ReadFileData(fileName string) error {
	var e error
	m.data, e = ioutil.ReadFile(fileName)
	m.FileSize = uint32(len(m.data))
	return e
}

var NAL3 []byte = []byte{0, 0, 1}
var NAL4 []byte = []byte{0, 0, 0, 1}

func (m *MediaFileSource) GetNextNalu() []byte {
	var findStartCode bool = false
	var i uint32 = 0
	for m.Offset < m.FileSize {
		if !findStartCode && bytes.Compare(NAL4, m.data[m.Offset:m.Offset+4]) == 0 {
			m.Offset += 4
			i = m.Offset
			findStartCode = true
		} else if !findStartCode && bytes.Compare(NAL3, m.data[m.Offset:m.Offset+3]) == 0 {
			m.Offset += 3
			i = m.Offset
			findStartCode = true
		} else if findStartCode && (bytes.Compare(NAL4, m.data[m.Offset:m.Offset+4]) == 0 || bytes.Compare(NAL3, m.data[m.Offset:m.Offset+3]) == 0) {
			sz := m.Offset - i
			rbytes := make([]byte, sz)
			copy(rbytes, m.data[i:m.Offset])
			return rbytes
		}
		m.Offset++
	}
	if findStartCode && i < m.FileSize {
		sz := m.Offset - i
		rbytes := make([]byte, sz)
		copy(rbytes, m.data[i:m.Offset])
		return rbytes
	}
	return nil
}

func (m *MediaFileSource) GetOneFrame() ([]byte, bool) {
	var iFrame bool = false
	var s []byte
	var findStartCode bool = false
	var endRead = false
	var i int = -1

	for m.Offset < m.FileSize {
		if !findStartCode && bytes.Compare(NAL4, m.data[m.Offset:m.Offset+4]) == 0 {
			if i == -1 {
				i = int(m.Offset)
			}
			m.Offset += 4
			tp := m.data[m.Offset] & 0x1f
			if tp == 5 {
				iFrame = true
			}
			if tp == 5 || tp == 1 || tp == 2 || tp == 3 || tp == 4 {
				endRead = true
			}
			findStartCode = true
		} else if !findStartCode && bytes.Compare(NAL3, m.data[m.Offset:m.Offset+3]) == 0 {
			if i == -1 {
				i = int(m.Offset)
			}
			m.Offset += 3
			tp := m.data[m.Offset] & 0x1f
			if tp == 5 {
				iFrame = true
			}
			if tp == 5 || tp == 1 || tp == 2 || tp == 3 || tp == 4 {
				endRead = true
			}
			findStartCode = true
		} else if findStartCode && (bytes.Compare(NAL4, m.data[m.Offset:m.Offset+4]) == 0 || bytes.Compare(NAL3, m.data[m.Offset:m.Offset+3]) == 0) {
			findStartCode = false
			if endRead {
				aul := []byte{0x00, 0x00, 0x00, 0x01, 0x09, 0x30}
				s = make([]byte, 6+m.Offset-uint32(i))
				copy(s, aul)
				copy(s[6:], m.data[i:])
				return s, iFrame
			} else {
				continue
			}
		}
		m.Offset++
	}
	if findStartCode && i < int(m.Offset) {
		aul := []byte{0x00, 0x00, 0x00, 0x01, 0x09, 0x30}
		s = make([]byte, 6+m.Offset-uint32(i))
		copy(s, aul)
		copy(s[6:], m.data[i:])
		return s, iFrame
	}
	return nil, iFrame
}

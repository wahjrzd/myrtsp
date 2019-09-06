// utility
package rtsp

import (
	"regexp"
	"strconv"
	"strings"
)

type RequestInfo struct {
	Method  string
	URL     string
	Version string
	Headers map[string]string
}

func ParseReqBuf(reqString string) *RequestInfo {
	lines := strings.Split(strings.TrimSpace(reqString), "\r\n")
	items := regexp.MustCompile("\\s+").Split(strings.TrimSpace(lines[0]), -1)
	if len(items) < 3 {
		return nil
	}
	header := make(map[string]string)
	for i := 1; i < len(lines); i++ {
		headItems := regexp.MustCompile(":\\s+").Split(lines[i], 2)
		if len(headItems) < 2 {
			continue
		}
		header[headItems[0]] = headItems[1]
	}
	return &RequestInfo{
		Method:  items[0],
		URL:     items[1],
		Version: items[2],
		Headers: header,
	}
}

type ResponseInfo struct {
	ResponseCode   int
	ResponseString string
	Headers        map[string]string
}

func parseRespBuf(respStr string) *ResponseInfo {
	lines := strings.Split(strings.TrimSpace(respStr), "\r\n")
	items := regexp.MustCompile("\\s+").Split(strings.TrimSpace(lines[0]), -1)
	if len(items) < 3 {
		return nil
	}
	header := make(map[string]string)

	for i := 1; i < len(lines); i++ {
		headItems := regexp.MustCompile(":\\s+").Split(lines[i], 2)
		if len(headItems) < 2 {
			continue
		}
		header[headItems[0]] = headItems[1]
	}
	rCode, _ := strconv.Atoi(items[1])
	return &ResponseInfo{
		ResponseCode:   rCode,
		ResponseString: items[2],
		Headers:        header,
	}
}

func parseWWWAuth(autStr string) (realm, nonce string) {
	realm = ""
	nonce = ""

	realmRex := regexp.MustCompile(`realm="(.*?)"`)
	result1 := realmRex.FindStringSubmatch(autStr)

	nonceRex := regexp.MustCompile(`nonce="(.*?)"`)
	result2 := nonceRex.FindStringSubmatch(autStr)

	if len(result1) == 2 {
		realm = result1[1]
	}
	if len(result2) == 2 {
		nonce = result2[1]
	}
	return
}

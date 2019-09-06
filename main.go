// rtspsrv
package main

import (
	rtsp "github.com/SimpleRtsp/rtsp"
)

func main() {
	rtsp := rtsp.NewRtspServer()
	if err := rtsp.GetConfig(); err != nil {
		return
	}
	if ok := rtsp.Start(); ok == false {
		return
	}
}

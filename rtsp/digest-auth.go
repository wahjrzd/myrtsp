// digest-auth
package rtsp

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
)

type DigestAuth struct {
	UserName string
	Password string
	Realm    string
	Nonce    string
}

func NewAuth() *DigestAuth {
	return &DigestAuth{}
}

func (auth *DigestAuth) CreateAuthenticatorString(cmd string, url string) string {
	if auth.Realm != "" && auth.UserName != "" && auth.Password != "" {
		if auth.Nonce != "" {
			resp := auth.computeDigestResponse(cmd, url)
			return fmt.Sprintf("Authorization: Digest username=\"%s\", realm=\"%s\",nonce=\"%s\", uri=\"%s\", response=\"%s\"\r\n",
				auth.UserName, auth.Realm, auth.Nonce, url, resp)
		} else {
			userPwd := fmt.Sprintf("%s:%s", auth.UserName, auth.Password)
			return fmt.Sprintf("Authorization: Basic %s\r\n", base64.StdEncoding.EncodeToString([]byte(userPwd)))
		}
	}
	return ""
}

func (auth *DigestAuth) computeDigestResponse(cmd string, url string) string {
	// The "response" field is computed as:
	//    md5(md5(<username>:<realm>:<password>):<nonce>:md5(<cmd>:<url>))
	// or, if "fPasswordIsMD5" is True:
	//    md5(<password>:<nonce>:md5(<cmd>:<url>))

	ha1Data := fmt.Sprintf("%s:%s:%s", auth.UserName, auth.Realm, auth.Password)
	m := md5.New()
	m.Write([]byte(ha1Data))
	ha1Buf := fmt.Sprintf("%x", m.Sum(nil))

	ha2Data := fmt.Sprintf("%s:%s", cmd, url)
	m.Reset()
	m.Write([]byte(ha2Data))
	ha2Buf := fmt.Sprintf("%x", m.Sum(nil))

	digestData := fmt.Sprintf("%s:%s:%s", ha1Buf, auth.Nonce, ha2Buf)
	m.Reset()
	m.Write([]byte(digestData))
	return fmt.Sprintf("%x", m.Sum(nil))
}

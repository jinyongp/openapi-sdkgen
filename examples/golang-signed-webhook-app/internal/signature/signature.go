package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"time"
)

func Sign(secret, id, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id + "." + timestamp + "."))
	_, _ = mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var timePattern = regexp.MustCompile(`^[0-9]{1,12}$`)
var signaturePattern = regexp.MustCompile(`^v1=[0-9a-f]{64}$`)

func Verify(secret, id, timestamp, signature string, body []byte) bool {
	if !idPattern.MatchString(id) || !timePattern.MatchString(timestamp) || !signaturePattern.MatchString(signature) {
		return false
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	delta := time.Now().Unix() - seconds
	if delta < -300 || delta > 300 {
		return false
	}
	received, _ := hex.DecodeString(signature[3:])
	expected, _ := hex.DecodeString(Sign(secret, id, timestamp, body)[3:])
	return hmac.Equal(received, expected)
}

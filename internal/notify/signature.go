package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"
)

// DingTalkSign 钉钉机器人加签
func DingTalkSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return url.QueryEscape(sign), nil
}

// FeishuSign 飞书机器人加签：HMAC-SHA256(key=secret, message=stringToSign)
func FeishuSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return sign, nil
}

// TimestampNow 返回当前秒级时间戳（飞书/钉钉 webhook 签名要求秒级）
func TimestampNow() int64 {
	return time.Now().Unix()
}

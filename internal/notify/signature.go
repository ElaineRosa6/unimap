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

// FeishuSign 飞书机器人加签：HMAC-SHA256(key=stringToSign, message=空)
// 飞书官方文档要求将 stringToSign 作为 HMAC 的 key，message 为空字节。
func FeishuSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	h.Write([]byte{})
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return sign, nil
}

// TimestampNow 返回当前秒级时间戳（飞书/钉钉 webhook 签名要求秒级）
func TimestampNow() int64 {
	return time.Now().Unix()
}

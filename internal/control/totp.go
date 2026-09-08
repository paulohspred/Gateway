package control

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="), nil
}

func TOTPURI(issuer, account, secret string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("period", "30")
	v.Set("digits", "6")
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}

func ValidateTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for delta := int64(-1); delta <= 1; delta++ {
		if subtleString(TOTPCode(secret, now.Add(time.Duration(delta)*30*time.Second)), code) {
			return true
		}
	}
	return false
}

func TOTPCode(secret string, now time.Time) string {
	padding := (8 - len(secret)%8) % 8
	decoded, err := base32.StdEncoding.DecodeString(strings.ToUpper(secret) + strings.Repeat("=", padding))
	if err != nil {
		return ""
	}
	counter := uint64(now.Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, decoded)
	_, _ = mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06s", strconv.FormatUint(uint64(binaryCode%1000000), 10))
}

func subtleString(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

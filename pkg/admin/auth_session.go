package admin

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const sessionPayloadVersion = "v2"

func formatSessionPayload(email string, issued, expires, idleDeadline int64) string {
	return fmt.Sprintf("%s|%s|%d|%d|%d", sessionPayloadVersion, email, issued, expires, idleDeadline)
}

func parseSessionPayload(signedValue string) (email string, issued, expires, idleDeadline int64, ok bool) {
	value, sigOK := verifySignature(signedValue)
	if !sigOK {
		return "", 0, 0, 0, false
	}
	if !strings.HasPrefix(value, sessionPayloadVersion+"|") {
		// Legacy: signed value is plain email.
		if value != "" && !strings.Contains(value, "|") {
			return value, 0, 0, 0, true
		}
		return "", 0, 0, 0, false
	}
	parts := strings.Split(value, "|")
	if len(parts) != 5 {
		return "", 0, 0, 0, false
	}
	issued, _ = strconv.ParseInt(parts[2], 10, 64)
	expires, _ = strconv.ParseInt(parts[3], 10, 64)
	idleDeadline, _ = strconv.ParseInt(parts[4], 10, 64)
	return parts[1], issued, expires, idleDeadline, true
}

func sessionPayloadExpired(expires, idleDeadline int64) bool {
	now := time.Now().Unix()
	if expires > 0 && now > expires {
		return true
	}
	if idleDeadline > 0 && now > idleDeadline {
		return true
	}
	return false
}

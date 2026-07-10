package sync

import (
	"errors"
	"io"
	"net"
	"strings"
)

// isTransient reports whether err is worth retrying (network / temporary server).
func isTransient(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	msg := strings.ToLower(err.Error())

	// Permanent: auth / policy — never retry.
	permanent := []string{
		"401",
		"403",
		"unauthorized",
		"forbidden",
		"not allowed",
		"authentication",
		"authorization failed",
		"pre-receive",
		"protected branch",
		"invalid token",
		"bad credentials",
	}
	for _, p := range permanent {
		if strings.Contains(msg, p) {
			return false
		}
	}

	transient := []string{
		"timeout",
		"timed out",
		"temporar",
		"connection refused",
		"connection reset",
		"connection abort",
		"broken pipe",
		"i/o timeout",
		"tls handshake",
		"network is unreachable",
		"no such host",
		"http2: client connection lost",
		"server misbehaving",
		"429",
		"502",
		"503",
		"504",
		"request timed out",
		"try again",
		"eof",
	}
	for _, t := range transient {
		if strings.Contains(msg, t) {
			return true
		}
	}
	return false
}

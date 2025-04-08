package socksproxy

import (
	"net"
	"strconv"

	"encoding/binary"
)

func toPort(port string) (buffer []byte) {
	buffer = make([]byte, 2)
	if port, err := strconv.ParseUint(port, 10, 16); err == nil {
		binary.BigEndian.PutUint16(buffer, uint16(port))
		return
	}
	return nil
}

func isZeros(p net.IP) bool {
	for i := 0; i < len(p); i++ {
		if p[i] != 0 {
			return false
		}
	}
	return true
}

func isIPv4(ip net.IP) bool {
	if len(ip) == net.IPv4len {
		return true
	}
	if len(ip) == net.IPv6len &&
		isZeros(ip[0:10]) &&
		ip[10] == 0xff &&
		ip[11] == 0xff {
		return true
	}
	return false
}

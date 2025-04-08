package socksproxy

import (
	"bufio"
	"encoding/binary"
	"net"
	"strconv"
)

// region SOCKS4

type socks4Request struct {
	command byte
	port    []byte
	ip      []byte
	userId  []byte
	fqdn    []byte
}

func (request *socks4Request) IsSOCKS4A() bool {
	ip := request.ip
	return ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] != 0
}
func (request *socks4Request) Address() string {
	var host, port string
	if request.IsSOCKS4A() {
		host = string(request.fqdn)
	} else {
		host = net.IP(request.ip).String()
	}
	port = strconv.Itoa(int(binary.BigEndian.Uint16(request.port)))
	return net.JoinHostPort(host, port)
}
func (request *socks4Request) ToPacket() []byte {
	packet := []byte{socks4version, request.command}
	packet = append(packet, request.port...)
	packet = append(packet, request.ip...)
	packet = append(packet, request.userId...)
	packet = append(packet, 0)
	if request.IsSOCKS4A() {
		packet = append(packet, request.fqdn...)
		packet = append(packet, 0)
	}
	return packet
}

type socks4Response struct {
	status byte
	port   []byte
	ip     []byte
}

func (response *socks4Response) ToPacket() []byte {
	packet := []byte{0x00, response.status}
	packet = append(packet, response.port...)
	packet = append(packet, response.ip...)
	return packet
}

func readSocks4Request(conn net.Conn) (request *socks4Request, err error) {
	reader := bufio.NewReader(conn)
	request = &socks4Request{}
	if request.command, err = reader.ReadByte(); err != nil {
		return
	}
	request.port = make([]byte, 2)
	if _, err = reader.Read(request.port); err != nil {
		return
	}
	request.ip = make([]byte, 4)
	if _, err = reader.Read(request.ip); err != nil {
		return
	}
	if request.userId, err = reader.ReadBytes(0); err != nil {
		return
	}
	if !request.IsSOCKS4A() {
		return
	}
	if request.fqdn, err = reader.ReadBytes(0); err != nil {
		return
	}
	return
}

// endregion

// region SOCKS5

type socks5Addr struct {
	addrType byte
	addr     []byte
	port     []byte
}

func readSocks5Addr(reader *bufio.Reader) (addr *socks5Addr, err error) {
	addr = &socks5Addr{}
	if addr.addrType, err = reader.ReadByte(); err != nil {
		return
	}
	switch addr.addrType {
	case socks5AddressTypeIPv4, socks5AddressTypeIPv6:
		length := net.IPv4len
		if addr.addrType == socks5AddressTypeIPv6 {
			length = net.IPv6len
		}
		addr.addr = make([]byte, length)
		if _, err = reader.Read(addr.addr); err != nil {
			return
		}
	case socks5AddressTypeFQDN:
		length, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		addr.addr = make([]byte, length)
		if _, err = reader.Read(addr.addr); err != nil {
			return nil, err
		}
	default:
		return nil, errAddressTypeNotSupported
	}
	addr.port = make([]byte, 2)
	if _, err = reader.Read(addr.port); err != nil {
		return
	}
	return
}

type socks5InitialRequest struct {
	version byte
	methods []byte
}

func (request *socks5InitialRequest) ToPacket() []byte {
	packet := []byte{
		request.version,
		byte(len(request.methods)),
	}
	packet = append(packet, request.methods...)
	return packet
}

type socks5InitialResponse []byte

func (response socks5InitialResponse) Length() int   { return len(response) }
func (response socks5InitialResponse) Version() byte { return response[0] }
func (response socks5InitialResponse) Auth() byte    { return response[1] }

type socks5Request struct {
	version byte
	command byte
	*socks5Addr
}

func (request *socks5Request) ToPacket() []byte {
	packet := []byte{
		request.version,
		request.command,
		0x00,
		request.addrType,
	}
	packet = append(packet, request.addr...)
	packet = append(packet, request.port...)
	return packet
}

func (request *socks5Request) Address() string {
	var host string
	switch request.addrType {
	case socks5AddressTypeIPv4, socks5AddressTypeIPv6:
		host = net.IP(request.addr).String()
	case socks5AddressTypeFQDN:
		host = string(request.addr)
	}
	port := strconv.Itoa(int(binary.BigEndian.Uint16(request.port)))
	return net.JoinHostPort(host, port)
}

func readSocks5Request(conn net.Conn) (request *socks5Request, err error) {
	reader := bufio.NewReader(conn)
	request = &socks5Request{}
	if request.version, err = reader.ReadByte(); err != nil {
		return
	}
	if request.command, err = reader.ReadByte(); err != nil {
		return
	}
	if _, err = reader.ReadByte(); err != nil {
		return
	}
	request.socks5Addr, err = readSocks5Addr(reader)
	return
}

// endregion

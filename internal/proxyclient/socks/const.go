package socksproxy

import "errors"

const (
	socks4version byte = 4
	socks5version byte = 5

	commandConnect      byte = 1
	commandUDPAssociate byte = 3

	socks4StatusGranted  byte = 90
	socks4StatusRejected byte = 91

	socks5AddressTypeIPv4 byte = 1
	socks5AddressTypeFQDN byte = 3
	socks5AddressTypeIPv6 byte = 4

	socks5StatusSucceeded               byte = 0
	socks5StatusGeneral                 byte = 1
	socks5StatusHostUnreachable         byte = 4
	socks5StatusConnectionRefused       byte = 5
	socks5StatusCommandNotSupported     byte = 7
	socks5StatusAddressTypeNotSupported byte = 8

	socks5AuthMethodNoRequired    byte = 0x00
	socks5AuthMethodPassword      byte = 0x02
	socks5AuthMethodTLSNoRequired byte = 0x80
	socks5AuthMethodTLSPassword   byte = 0x82
	socks5AuthMethodNoAcceptable  byte = 0xFF
)

var (
	errVersionError            = errors.New("version error")
	errCommandNotSupported     = errors.New("command not supported")
	errAddressTypeNotSupported = errors.New("address type not supported")
	errAuthMethodNotSupported  = errors.New("authentication method not supported")
)

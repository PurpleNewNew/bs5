package loadbalance

import (
	"context"
	"hash/crc32"
	"net"

	"github.com/PurpleNewNew/bs5/internal/proxyclient"
)

func NewHash(proxies []proxyclient.Dial) proxyclient.Dial {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		checksum := crc32.ChecksumIEEE([]byte(address))
		dial := proxies[int(checksum)%len(proxies)]
		return dial(ctx, network, address)
	}
}

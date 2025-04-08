package loadbalance

import (
	"context"
	"github.com/PurpleNewNew/bs5/internal/proxyclient"
	"net"
)

func NewRoundRobin(proxies []proxyclient.Dial) proxyclient.Dial {
	index := 0
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if int(index) > len(proxies) {
			index = 0
		}
		dial := proxies[index]
		index += 1
		return dial(ctx, network, address)
	}
}

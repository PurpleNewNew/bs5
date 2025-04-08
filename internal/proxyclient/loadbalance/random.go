package loadbalance

import (
	"context"
	"math/rand"
	"net"

	"github.com/PurpleNewNew/bs5/internal/proxyclient"
)

func NewRandom(proxies []proxyclient.Dial) proxyclient.Dial {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dial := proxies[rand.Intn(len(proxies))]
		return dial(ctx, network, address)
	}
}

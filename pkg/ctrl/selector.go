package ctrl

import (
	"net"
	"net/url"

	"github.com/go-gost/gosocks5"
	log "github.com/kataras/golog"
)

// customClientSelector is a custom implementation of the SOCKS5 client selector
// that correctly handles the NoAuth method, unlike the default selector in v0.4.2.
type customClientSelector struct {
	methods []uint8
	user    *url.Userinfo
}

// NewCustomClientSelector creates a new custom client selector.
func NewCustomClientSelector(user *url.Userinfo, methods ...uint8) gosocks5.Selector {
	// If no methods are specified, default to supporting NoAuth and UserPass.
	if len(methods) == 0 {
		methods = []uint8{gosocks5.MethodNoAuth, gosocks5.MethodUserPass}
	}
	return &customClientSelector{
		methods: methods,
		user:    user,
	}
}

func (selector *customClientSelector) Methods() []uint8 {
	log.Debugf("Client selector: advertising methods: %v", selector.methods)
	return selector.methods
}

func (selector *customClientSelector) Select(methods ...uint8) (method uint8) {
	log.Debugf("Client selector: server supports methods: %v", methods)

	// If we have user credentials, prefer UserPass
	if selector.user != nil {
		for _, m := range methods {
			if m == gosocks5.MethodUserPass {
				log.Debugf("Client selector: choosing MethodUserPass")
				return gosocks5.MethodUserPass
			}
		}
	}

	// Otherwise, prefer NoAuth
	for _, m := range methods {
		if m == gosocks5.MethodNoAuth {
			log.Debugf("Client selector: choosing MethodNoAuth")
			return gosocks5.MethodNoAuth
		}
	}

	log.Debugf("Client selector: no suitable method found")
	return gosocks5.MethodNoAcceptable
}

// OnSelected is the corrected implementation that handles NoAuth.
func (selector *customClientSelector) OnSelected(method uint8, conn net.Conn) (string, net.Conn, error) {
	log.Debugf("Client selector: server selected method: %d", method)
	switch method {
	case gosocks5.MethodNoAuth:
		log.Debugf("Client selector: using NoAuth, no further authentication needed")
		return "", conn, nil // This case was missing in the library's default selector
	case gosocks5.MethodUserPass:
		var username, password string
		if selector.user != nil {
			username = selector.user.Username()
			password, _ = selector.user.Password()
		}
		log.Debugf("Client selector: sending UserPass auth for user: %s", username)

		req := gosocks5.NewUserPassRequest(gosocks5.UserPassVer, username, password)
		if err := req.Write(conn); err != nil {
			log.Errorf("Client selector: failed to write UserPass request: %v", err)
			return "", nil, err
		}
		resp, err := gosocks5.ReadUserPassResponse(conn)
		if err != nil {
			log.Errorf("Client selector: failed to read UserPass response: %v", err)
			return "", nil, err
		}
		if resp.Status != gosocks5.Succeeded {
			log.Errorf("Client selector: UserPass auth failed with status: %d", resp.Status)
			return "", nil, gosocks5.ErrAuthFailure
		}
		log.Debugf("Client selector: UserPass auth succeeded")
		return "", conn, nil

	case gosocks5.MethodNoAcceptable:
		log.Errorf("Client selector: no acceptable authentication method")
		return "", nil, gosocks5.ErrBadMethod
	default:
		log.Errorf("Client selector: unknown method: %d", method)
		return "", nil, gosocks5.ErrBadFormat
	}
}

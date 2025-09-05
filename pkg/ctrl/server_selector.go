package ctrl

import (
	"net"
	"net/url"

	"github.com/go-gost/gosocks5"
)

// serverSelector is a SOCKS5 server selector.
type serverSelector struct {
	methods []uint8
	user    *url.Userinfo
}

// NewServerSelector creates a new server selector.
func NewServerSelector(user *url.Userinfo, methods ...uint8) gosocks5.Selector {
	if len(methods) == 0 {
		methods = []uint8{gosocks5.MethodNoAuth, gosocks5.MethodUserPass}
	}
	return &serverSelector{
		methods: methods,
		user:    user,
	}
}

func (selector *serverSelector) Methods() []uint8 {
	return selector.methods
}

func (selector *serverSelector) Select(methods ...uint8) (method uint8) {
	// if user is specified, user/pass auth is mandatory
	if selector.user != nil {
		for _, m := range methods {
			if m == gosocks5.MethodUserPass {
				return gosocks5.MethodUserPass
			}
		}
	}

	// If the required method is not supported, and NoAuth is supported, we select it.
	for _, m := range methods {
		if m == gosocks5.MethodNoAuth {
			return gosocks5.MethodNoAuth
		}
	}

	return gosocks5.MethodNoAcceptable
}

// OnSelected is called after a method is selected.
func (selector *serverSelector) OnSelected(method uint8, conn net.Conn) (string, net.Conn, error) {
	switch method {
	case gosocks5.MethodUserPass:
		req, err := gosocks5.ReadUserPassRequest(conn)
		if err != nil {
			return "", nil, err
		}

		var serverUsername, serverPassword string
		if selector.user != nil {
			serverUsername = selector.user.Username()
			serverPassword, _ = selector.user.Password()
		}

		if req.Username != serverUsername || req.Password != serverPassword {
			resp := gosocks5.NewUserPassResponse(gosocks5.UserPassVer, gosocks5.Failure)
			if err := resp.Write(conn); err != nil {
				return "", nil, err
			}
			return "", nil, gosocks5.ErrAuthFailure
		}

		resp := gosocks5.NewUserPassResponse(gosocks5.UserPassVer, gosocks5.Succeeded)
		if err := resp.Write(conn); err != nil {
			return "", nil, err
		}

		// Return the username as the clientID
		return req.Username, conn, nil

	case gosocks5.MethodNoAuth:
		// No auth, no further action needed
		return "", conn, nil

	default:
		return "", nil, gosocks5.ErrBadMethod
	}
}

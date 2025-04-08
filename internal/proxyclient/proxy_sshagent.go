package proxyclient

import (
	"context"
	"errors"
	"io/ioutil"
	"net/url"
	"sync"

	"golang.org/x/crypto/ssh"
)

type sshClientCache struct {
	sync.RWMutex
	clients map[string]*ssh.Client
}

var (
	globalSSHCache = &sshClientCache{
		clients: make(map[string]*ssh.Client),
	}
)

func (c *sshClientCache) getClient(key string) *ssh.Client {
	c.RLock()
	defer c.RUnlock()
	return c.clients[key]
}

func (c *sshClientCache) setClient(key string, client *ssh.Client) {
	c.Lock()
	defer c.Unlock()
	c.clients[key] = client
}

func newSSHProxyClient(proxy *url.URL, upstreamDial Dial) (dial Dial, err error) {
	if proxy.User == nil {
		err = errors.New("must set username")
		return
	}

	cacheKey := proxy.String()

	if client := globalSSHCache.getClient(cacheKey); client != nil {
		return WrapDialerContext(client.Dial).TCPOnly, nil
	}

	auth, err := sshAuth(proxy)
	if err != nil {
		return nil, err
	}
	conf := &ssh.ClientConfig{
		User:            proxy.User.Username(),
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := upstreamDial(context.Background(), "tcp", proxy.Host)
	if err != nil {
		return
	}
	sshConn, sshChans, sshRequests, err := ssh.NewClientConn(conn, proxy.Host, conf)
	if err != nil {
		return
	}
	sshClient := ssh.NewClient(sshConn, sshChans, sshRequests)

	globalSSHCache.setClient(cacheKey, sshClient)

	dial = WrapDialerContext(sshClient.Dial).TCPOnly
	return
}

func sshAuth(proxy *url.URL) ([]ssh.AuthMethod, error) {
	methods := []ssh.AuthMethod{}
	publicKey := proxy.Query().Get("public-key")
	if publicKey != "" {
		buffer, err := ioutil.ReadFile(publicKey)
		if err != nil {
			return nil, err
		}
		key, err := ssh.ParsePrivateKey(buffer)
		if err != nil {
			return nil, err
		}
		method := ssh.PublicKeys(key)
		methods = append(methods, method)
	}
	if password, ok := proxy.User.Password(); ok {
		method := ssh.Password(password)
		methods = append(methods, method)
	}
	return methods, nil
}

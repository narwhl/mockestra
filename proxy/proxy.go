package proxy

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/docker/go-connections/nat"
)

// Option configures the behavior of a TCPProxy created by NewProxy.
type Option func(*proxyConfig)

type proxyConfig struct {
	listenPort string
}

// WithListenPort overrides the local port the proxy listens on.
// By default, the proxy listens on the same port number as the container's
// exposed port. Use this option when the proxy must bind to a specific local
// port that differs from the container port (e.g., a pre-allocated ephemeral port).
//
// Example:
//
//	concourse.NewProxy("API", nat.Port(concourse.Port), proxy.WithListenPort(58033))
func WithListenPort(port int) Option {
	return func(c *proxyConfig) {
		c.listenPort = strconv.Itoa(port)
	}
}

// ResolveListenPort determines the local port for the proxy to listen on.
// It applies the given options and returns the overridden port if [WithListenPort]
// was provided, otherwise falls back to the port number from containerPort.
func ResolveListenPort(containerPort nat.Port, opts ...Option) string {
	cfg := &proxyConfig{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.listenPort != "" {
		return cfg.listenPort
	}
	return containerPort.Port()
}

type TCPProxy struct {
	ListenAddress string
	TargetAddress string
	listener      net.Listener
	cancel        context.CancelFunc
}

func (p *TCPProxy) handleConnection(upstreamConn net.Conn) error {
	downstreamConn, err := net.Dial("tcp", p.TargetAddress)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(downstreamConn, upstreamConn)
	}()
	go func() {
		defer wg.Done()
		io.Copy(upstreamConn, downstreamConn)
	}()
	wg.Wait()
	if err := downstreamConn.Close(); err != nil {
		return err
	}
	if err := upstreamConn.Close(); err != nil {
		return err
	}
	return nil
}

func (p *TCPProxy) Start(ctx context.Context) error {
	if p.listener != nil {
		return nil
	}
	listener, err := net.Listen("tcp", p.ListenAddress)
	if err != nil {
		return err
	}
	p.listener = listener
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	p.cancel = cancelFunc
	go p.Run(cancelCtx)
	return nil
}

func (p *TCPProxy) Close(ctx context.Context) error {
	p.cancel()
	if p.listener == nil {
		return nil
	}
	return p.listener.Close()
}

func (p *TCPProxy) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := p.listener.Accept()
			if err != nil {
				return err
			}
			go p.handleConnection(conn)
		}
	}
}

package proxy

import (
	"context"
	"io"
	"net"
	"sync"
)

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

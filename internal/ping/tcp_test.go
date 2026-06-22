package ping

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestPingIPWithTCPOptions(t *testing.T) {
	oldDial := tcpDialContext
	defer func() {
		tcpDialContext = oldDial
	}()
	var addresses []string
	tcpDialContext = func(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
		addresses = append(addresses, address)
		time.Sleep(time.Millisecond)
		client, server := net.Pipe()
		go func() {
			_ = server.Close()
		}()
		return client, nil
	}

	result := PingIPWithOptionsContext(context.Background(), "127.0.0.1", Options{
		Method:   "tcp",
		TCPPort:  8443,
		Count:    2,
		Interval: time.Millisecond,
		Timeout:  time.Second,
	})

	if result.Error != nil {
		t.Fatalf("expected tcp ping to succeed: %v", result.Error)
	}
	if result.RTT <= 0 {
		t.Fatalf("expected positive RTT, got %s", result.RTT)
	}
	if result.PacketsSent != 2 || result.PacketsRecv != 2 {
		t.Fatalf("expected all tcp attempts to succeed, got sent=%d recv=%d", result.PacketsSent, result.PacketsRecv)
	}
	if result.PacketLoss != 0 {
		t.Fatalf("expected zero packet loss, got %.1f", result.PacketLoss)
	}
	if len(addresses) != 2 || addresses[0] != "127.0.0.1:8443" || addresses[1] != "127.0.0.1:8443" {
		t.Fatalf("unexpected dial addresses: %#v", addresses)
	}
}

func TestPingIPWithTCPOptionsFailsWithoutRTT(t *testing.T) {
	oldDial := tcpDialContext
	defer func() {
		tcpDialContext = oldDial
	}()
	tcpDialContext = func(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
		return nil, errors.New("connection refused")
	}

	result := PingIPWithOptionsContext(context.Background(), "127.0.0.1", Options{
		Method:   "tcp",
		TCPPort:  8443,
		Count:    1,
		Timeout:  50 * time.Millisecond,
		Interval: time.Millisecond,
	})

	if result.Error == nil {
		t.Fatal("expected tcp ping to fail")
	}
	if result.RTT != 0 {
		t.Fatalf("expected failed tcp ping to have no RTT, got %s", result.RTT)
	}
	if result.PacketsSent != 1 || result.PacketsRecv != 0 {
		t.Fatalf("expected failed tcp attempt stats, got sent=%d recv=%d", result.PacketsSent, result.PacketsRecv)
	}
	if result.PacketLoss != 100 {
		t.Fatalf("expected 100%% packet loss, got %.1f", result.PacketLoss)
	}
}

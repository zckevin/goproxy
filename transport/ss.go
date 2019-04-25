package transport

import (
    "time"
    "net"
    "github.com/shadowsocks/go-shadowsocks2/socks"
    "github.com/shadowsocks/go-shadowsocks2/core"
)

type ServerConfig struct {
	Server string
	// cihper *Cipher
	Method   string
	Password string
}

var dialer *net.Dialer

func init() {
    dialer = &net.Dialer{
        Timeout:   3 * time.Second,
        KeepAlive: 60 * time.Second,
    }
}

func CreateTCPConn(sc ServerConfig, targetAddr string) (net.Conn, error) {
    // fmt.Println(sc, targetAddr)
	cipher, err := core.PickCipher(sc.Method, nil, sc.Password)
	if err != nil {
		return nil, err
	}

    // remote, err := net.Dial("tcp", sc.Server)
    remote, err := dialer.Dial("tcp", sc.Server)
    if err != nil {
        return nil, err
    }
    // defer remote.Close()

    remote.(*net.TCPConn).SetKeepAlive(true)
    remote = cipher.StreamConn(remote)

	if _, err = remote.Write(socks.ParseAddr(targetAddr)); err != nil {
	    return nil, err
	}

	return remote, nil
}

func CreateUDPConn(sc ServerConfig) (net.PacketConn, error) {
	cipher, err := core.PickCipher(sc.Method, nil, sc.Password)
	if err != nil {
		return nil, err
	}

    remote, err := net.ListenPacket("udp", "")
    // remote, err := net.Dial("udp", sc.Server)

    if err != nil {
        return nil, err
    }

    return cipher.PacketConn(remote), nil
}

package privnet

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func AllPeers(ctx context.Context, appName string) ([]net.IPAddr, error) {
	return Get6PN(ctx, fmt.Sprintf("%s.internal", appName))
}

func Get6PN(ctx context.Context, hostname string) ([]net.IPAddr, error) {
	nameserver := os.Getenv("FLY_NAMESERVER")
	if nameserver == "" {
		nameserver = "fdaa::3"
	}
	nameserver = net.JoinHostPort(nameserver, "53")
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 1 * time.Second,
			}
			return d.DialContext(ctx, "udp6", nameserver)
		},
	}
	ips, err := r.LookupIPAddr(ctx, hostname)

	if err != nil {
		return ips, err
	}

	// make sure we're including the local ip, just in case it's not in service discovery yet
	local, err := r.LookupIPAddr(ctx, "fly-local-6pn")

	if err != nil || len(local) < 1 {
		return ips, err
	}

	localExists := false
	for _, v := range ips {
		if v.IP.String() == local[0].IP.String() {
			localExists = true
		}
	}

	if !localExists {
		ips = append(ips, local[0])
	}
	return ips, err
}

func PrivateIPv6() (net.IP, error) {
	ips, err := net.LookupIP("fly-local-6pn")
	if err != nil && !strings.HasSuffix(err.Error(), "no such host") && !strings.HasSuffix(err.Error(), "server misbehaving") {
		return nil, err
	}

	if len(ips) > 0 {
		return ips[0], nil
	}

	return net.ParseIP("127.0.0.1"), nil
}

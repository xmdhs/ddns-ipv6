package ipv6

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

func GetIpv6(ctx context.Context) ([]netip.Addr, error) {
	as, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	ipv6s := []netip.Addr{}
	for _, v := range as {
		p := netip.MustParsePrefix(v.String())
		ip := p.Addr()
		if !ip.Is6() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsMulticast() {
			continue
		}
		ipv6s = append(ipv6s, ip)
	}
	if len(ipv6s) == 0 {
		return nil, fmt.Errorf("GetIpv6: %w", ErrNotIpv6)
	}

	outIP, err := getOutIpv6()
	if err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	if len(ipv6s) > 1 {
		for _, v := range ipv6s {
			if v != outIP {
				ipv6s = append(ipv6s, v)
			}
		}
	}
	return ipv6s, nil
}

var ErrNotIpv6 = errors.New("ErrNotIpv6")

func getOutIpv6() (netip.Addr, error) {
	l, err := net.Dial("udp6", "[2001:4860:4860::8844]:53")
	if err != nil {
		return netip.Addr{}, fmt.Errorf("getLocal: %w", err)
	}
	defer l.Close()
	return netip.MustParseAddrPort(l.LocalAddr().String()).Addr(), nil
}

package netlink

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"

	"github.com/vishvananda/netlink"
)

func Subscribe(ctx context.Context, f func()) error {
	addrCH := make(chan netlink.AddrUpdate, 10)
	done := make(chan struct{})

	context.AfterFunc(ctx, func() {
		close(done)
	})

	err := netlink.AddrSubscribe(addrCH, done)
	if err != nil {
		return fmt.Errorf("Subscribe: %w", err)
	}

	for range addrCH {
		f()
	}
	return nil
}

const IFA_F_TEMPORARY = 0x01

func GetIpv6(ctx context.Context) ([]netip.Addr, error) {
	r, err := netlink.RouteGet(net.ParseIP("2001:4860:4860::8844"))
	if err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	if len(r) == 0 {
		return nil, fmt.Errorf("not route")
	}
	link, err := netlink.LinkByIndex(r[0].LinkIndex)
	if err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	addr, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	raddr := make([]netip.Addr, 0)

	sort.Slice(addr, func(i, j int) bool {
		return addr[i].PreferedLft > addr[j].PreferedLft
	})

	for _, v := range addr {
		if v.Flags&IFA_F_TEMPORARY != 0 {
			continue
		}
		ip, err := netip.ParseAddr(v.IP.String())
		if err != nil {
			panic(err)
		}
		if ip.IsGlobalUnicast() {
			raddr = append(raddr, ip)
		}
	}
	return raddr, nil
}

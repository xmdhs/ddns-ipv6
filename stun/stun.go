package stun

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/pion/stun"
)

func GetIp(ctx context.Context, ipv6 bool) ([]netip.Addr, error) {
	d := net.Dialer{}
	network := "udp4"
	if ipv6 {
		network = "udp6"
	}
	sc, err := d.DialContext(ctx, network, "stun.cloudflare.com:3478")
	if err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	defer sc.Close()

	c, err := stun.NewClient(sc)
	if err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	defer c.Close()

	var xorAddr stun.XORMappedAddress
	var errr error

	if err = c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			errr = res.Error
			return
		}
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			errr = getErr
			return
		}
	}); err != nil {
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	if errr != nil {
		return nil, fmt.Errorf("GetIpv6: %w", errr)
	}
	return []netip.Addr{netip.MustParseAddr(xorAddr.IP.String())}, nil
}

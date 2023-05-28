package ipv6stun

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/pion/stun"
)

func GetIpv6(ctx context.Context) ([]netip.Addr, error) {
	d := net.Dialer{}
	sc, err := d.DialContext(ctx, "udp6", "stun.cloudflare.com:3478")
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
		return nil, fmt.Errorf("GetIpv6: %w", err)
	}
	return []netip.Addr{netip.MustParseAddr(xorAddr.IP.String())}, nil
}

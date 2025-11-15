package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/netip"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cloudflare/cloudflare-go"
	"github.com/joho/godotenv"
	"github.com/xmdhs/ddns-ipv6/ipv6"
	"github.com/xmdhs/ddns-ipv6/netlink"
	s "github.com/xmdhs/ddns-ipv6/stun"
)

var (
	wlanIP  string
	domain  string
	zoneID  string
	gettype string
	test    bool
	domain4 string
)

func init() {
	flag.StringVar(&wlanIP, "w", "", "")
	flag.StringVar(&domain, "d", "", "")
	flag.StringVar(&domain4, "d4", "", "")
	flag.StringVar(&zoneID, "z", "", "")
	flag.StringVar(&gettype, "t", "netlink", "")
	flag.BoolVar(&test, "test", false, "")
	flag.Parse()
}

func main() {
	godotenv.Load()
	cftoken := os.Getenv("cf_token")
	cxt := context.Background()

	var f func(ctx context.Context) ([]netip.Addr, error)
	switch gettype {
	case "stun":
		f = func(ctx context.Context) ([]netip.Addr, error) { return s.GetIp(ctx, true) }
	case "interface":
		f = ipv6.GetIpv6
	case "netlink":
		f = netlink.GetIpv6
	}

	if test {
		ips, err := f(cxt)
		if err != nil {
			panic(err)
		}
		fmt.Println(ips)
		ips, err = s.GetIp(cxt, false)
		if err != nil {
			panic(err)
		}
		fmt.Println(ips)
	}

	if domain4 != "" {
		go func() {
			ipv4 := netip.Addr{}
			for {
				func() {
					cxt, c := context.WithTimeout(cxt, 2*time.Minute)
					defer c()
					ipv4 = retrySetDns(cxt, cftoken, func(ctx context.Context) ([]netip.Addr, error) { return s.GetIp(cxt, false) }, true, ipv4)
					time.Sleep(1 * time.Minute)
				}()
			}
		}()
	}

	ipv6 := retrySetDns(cxt, cftoken, f, true, netip.Addr{})
	if gettype == "netlink" {
		netlink.Subscribe(cxt, func() {
			ipv6 = retrySetDns(cxt, cftoken, f, true, ipv6)
		})
	} else {
		for {
			func() {
				cxt, c := context.WithTimeout(cxt, 2*time.Minute)
				defer c()
				ipv6 = retrySetDns(cxt, cftoken, f, true, ipv6)
				time.Sleep(1 * time.Minute)
			}()
		}
	}
}

func retrySetDns(cxt context.Context, cftoken string, getfunc func(ctx context.Context) ([]netip.Addr, error), ipv6 bool, oldIp netip.Addr) netip.Addr {
	ip, err := retry.DoWithData[netip.Addr](func() (netip.Addr, error) {
		return doSome(cxt, cftoken, getfunc, ipv6, oldIp)
	}, retryOpts...)
	if err != nil {
		log.Println(err)
	}
	return ip
}

func doSome(cxt context.Context, cftoken string, getfunc func(ctx context.Context) ([]netip.Addr, error), ipv6 bool, oldIp netip.Addr) (netip.Addr, error) {
	ip, err := getfunc(cxt)
	if err != nil {
		return netip.Addr{}, err
	}
	if ip[0] == oldIp {
		return ip[0], nil
	}

	capi, err := cloudflare.NewWithAPIToken(cftoken)
	if err != nil {
		return netip.Addr{}, err
	}

	t := "A"
	if ipv6 {
		t = "AAAA"
	}

	records, _, err := capi.ListDNSRecords(cxt, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Name: domain,
		Type: t,
	})
	if err != nil {
		return netip.Addr{}, err
	}
	if len(records) < 1 {
		panic("没有找到这个域名")
	}

	r := records[0]
	rip := netip.MustParseAddr(r.Content)
	if ip[0] == rip {
		return netip.Addr{}, nil
	}

	nip := ip[0].String()

	_, err = capi.UpdateDNSRecord(cxt, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
		Type:    t,
		Name:    domain,
		Content: nip,
		ID:      r.ID,
	})
	if err != nil {
		return netip.Addr{}, err
	}
	log.Println(domain, "已修改为", nip)
	return ip[0], nil
}

var retryOpts = []retry.Option{
	retry.Attempts(0),
	retry.LastErrorOnly(true),
	retry.OnRetry(func(n uint, err error) {
		log.Printf("retry %d: %v", n, err)
	}),
}

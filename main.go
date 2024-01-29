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
	"github.com/xmdhs/ddns-ipv6/ipv6stun"
	"github.com/xmdhs/ddns-ipv6/netlink"
)

var (
	wlanIP  string
	domain  string
	zoneID  string
	gettype string
	test    bool
)

func init() {
	flag.StringVar(&wlanIP, "w", "", "")
	flag.StringVar(&domain, "d", "", "")
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
		f = ipv6stun.GetIpv6
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
	}

	if gettype == "netlink" {
		netlink.Subscribe(cxt, func() {
			retrySetDns(cxt, cftoken, f)
		})
	} else {
		for {
			func() {
				cxt, c := context.WithTimeout(cxt, 2*time.Minute)
				defer c()
				retrySetDns(cxt, cftoken, f)
				time.Sleep(3 * time.Minute)
			}()
		}
	}
}

func retrySetDns(cxt context.Context, cftoken string, getfunc func(ctx context.Context) ([]netip.Addr, error)) {
	err := retry.Do(func() error {
		return doSome(cxt, cftoken, getfunc)
	}, retryOpts...)
	if err != nil {
		log.Println(err)
	}
}

func doSome(cxt context.Context, cftoken string, getfunc func(ctx context.Context) ([]netip.Addr, error)) error {
	capi, err := cloudflare.NewWithAPIToken(cftoken)
	if err != nil {
		return err
	}

	records, _, err := capi.ListDNSRecords(cxt, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Name: domain,
		Type: "AAAA",
	})
	if err != nil {
		return err
	}
	if len(records) < 1 {
		panic("没有找到这个域名")
	}

	ip, err := getfunc(cxt)
	if err != nil {
		return err
	}

	r := records[0]
	rip := netip.MustParseAddr(r.Content)
	if ip[0] == rip {
		return nil
	}

	nip := ip[0].String()

	_, err = capi.UpdateDNSRecord(cxt, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
		Type:    "AAAA",
		Name:    domain,
		Content: nip,
		ID:      r.ID,
	})
	if err != nil {
		return err
	}
	log.Println(domain, "已修改为", nip)
	return nil
}

var retryOpts = []retry.Option{
	retry.Attempts(0),
	retry.LastErrorOnly(true),
	retry.Delay(time.Second * 3),
	retry.MaxDelay(5 * time.Minute),
	retry.OnRetry(func(n uint, err error) {
		log.Printf("retry %d: %v", n, err)
	}),
}

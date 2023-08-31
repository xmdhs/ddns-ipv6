package main

import (
	"context"
	"flag"
	"log"
	"net/netip"
	"os"
	"time"

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
)

func init() {
	flag.StringVar(&wlanIP, "w", "", "")
	flag.StringVar(&domain, "d", "", "")
	flag.StringVar(&zoneID, "z", "", "")
	flag.StringVar(&zoneID, "t", "netlink", "")
	flag.Parse()
}

func main() {
	godotenv.Load()
	cftoken := os.Getenv("cf_token")
	cxt := context.Background()

	if gettype == "netlink" {
		netlink.Subscribe(cxt, func() {
			doSome(cxt, cftoken, netlink.GetIpv6)
		})
	} else {
		var f func(ctx context.Context) ([]netip.Addr, error)
		switch gettype {
		case "stun":
			f = ipv6stun.GetIpv6
		case "interface":
			f = ipv6.GetIpv6
		}
		func() {
			cxt, c := context.WithTimeout(cxt, 2*time.Minute)
			defer c()
			doSome(cxt, cftoken, f)
			time.Sleep(3 * time.Minute)
		}()
	}
}

func doSome(cxt context.Context, cftoken string, getfunc func(ctx context.Context) ([]netip.Addr, error)) {
	capi, err := cloudflare.NewWithAPIToken(cftoken)
	if err != nil {
		log.Println(err)
		return
	}

	records, _, err := capi.ListDNSRecords(cxt, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Name: domain,
		Type: "AAAA",
	})
	if err != nil {
		log.Println(err)
		return
	}
	if len(records) < 1 {
		panic("没有找到这个域名")
	}

	ip, err := getfunc(cxt)
	if err != nil {
		log.Println(err)
		return
	}

	r := records[0]
	rip := netip.MustParseAddr(r.Content)

	for _, v := range ip {
		if v == rip {
			return
		}
	}
	nip := ip[0].String()

	_, err = capi.UpdateDNSRecord(cxt, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
		Type:    "AAAA",
		Name:    domain,
		Content: nip,
		ID:      r.ID,
	})
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(domain, "已修改为", nip)
}

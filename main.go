package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

var (
	addr      string
	port      string
	protocols = stringList{"tcp", "udp"}
	domains   = stringList{"."}
	ttl       = uint64(60)
	mbox      string
	serial    uint64
	refresh   = uint64(3600)
	retry     = uint64(900)
	expire    = uint64(604800)
	minttl    = uint64(3600)
	adminUser = "admin"
)

type stringList []string

func (s *stringList) Set(value string) error {
	if value == "" {
		return nil
	}
	*s = strings.Split(value, ",")
	return nil
}

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func init() {
	flag.StringVar(&addr, "addr", "", "listen address")
	flag.StringVar(&port, "port", "53", "listen port")
	flag.Var(&protocols, "proto", "listen protocol list")
	flag.Var(&domains, "domain", "domain list")
	flag.Uint64Var(&ttl, "ttl", ttl, "TTL")
	flag.StringVar(&mbox, "mbox", adminUser+"@<domain>", "SOA mbox")
	flag.Uint64Var(&serial, "serial", serial, "SOA serial")
	flag.Uint64Var(&refresh, "refresh", refresh, "SOA refresh")
	flag.Uint64Var(&expire, "expire", expire, "SOA expire")
	flag.Uint64Var(&minttl, "minttl", minttl, "SOA minttl")
}

func main() {
	flag.Parse()
	if mbox == "" {
		mbox = adminUser
	}

	g := errgroup.Group{}
	for _, proto := range protocols {
		for _, domain := range domains {
			server := &dns.Server{
				Addr: addr + ":" + port,
				Net:  proto,
			}
			dns.HandleFunc(domain, handleRequest(domain))
			g.Go(server.ListenAndServe)
			log.Println("INFO\tlisten domain=%s", server.Net, server.Addr, domain)
		}
	}
	fmt.Println("")
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func handleRequest(domain string) dns.HandlerFunc {
	mbox := strings.Replace(mbox, "@", ".", -1)
	if mbox == "" {
		mbox = adminUser + "." + domain
	}
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		for _, q := range r.Question {
			log.Printf("INFO [%04x]\trequest: remote=%v qtype=%v name=%v\n", r.MsgHdr.Id, w.RemoteAddr(), dns.TypeToString[q.Qtype], q.Name)
			if q.Qclass != dns.ClassINET {
				log.Printf("ERROR[%04x]\tunsupported qclass: %s\n", r.MsgHdr.Id, dns.ClassToString[q.Qclass])
				continue
			}

			switch q.Qtype {
			case dns.TypeA:
				req := net.ParseIP(strings.TrimSuffix(q.Name, "."+domain)).To4()
				if req == nil || req.IsUnspecified() {
					log.Printf("ERROR[%04x]\tinvalid domain name: %s\n", r.MsgHdr.Id, q.Name)
					continue
				}

				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    uint32(ttl),
					},
					A: req,
				})
			case dns.TypeNS:
				m.Answer = append(m.Answer, &dns.NS{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeNS,
						Class:  dns.ClassINET,
						Ttl:    uint32(ttl),
					},
					Ns: domain,
				})
			case dns.TypeSOA:
				m.Answer = append(m.Answer, &dns.SOA{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeSOA,
						Class:  dns.ClassINET,
						Ttl:    uint32(ttl),
					},
					Ns:      domain,
					Mbox:    mbox,
					Serial:  uint32(serial),
					Refresh: uint32(refresh),
					Retry:   uint32(retry),
					Expire:  uint32(expire),
					Minttl:  uint32(minttl),
				})
			default:
				log.Printf("ERROR[%04x]\tunsupported qtype: %s\n", r.MsgHdr.Id, dns.TypeToString[q.Qtype])
			}

		}
		if err := w.WriteMsg(m); err != nil {
			log.Printf("ERROR[%04x]\t %v", r.MsgHdr.Id, err)
		}
	}
}

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

var (
	addr      string
	port      string
	protocols []string
	domain    string

	ttl     = uint32(60)
	mbox    string
	serial  uint32
	refresh = uint32(3600)
	retry   = uint32(900)
	expire  = uint32(604800)
	minttl  = uint32(3600)
)

func main() {
	addr = os.Getenv("ADDR")

	port = os.Getenv("PORT")
	if port == "" {
		port = "53"
	}

	proto := os.Getenv("PROTO")
	if proto == "" {
		protocols = []string{"tcp", "udp"}
	} else {
		protocols = strings.Split(proto, ",")
	}

	domain = dns.Fqdn(os.Getenv("DOMAIN"))
	mbox = dns.Fqdn("admin." + domain)

	g := errgroup.Group{}
	for _, proto := range protocols {
		server := &dns.Server{
			Addr: addr + ":" + port,
			Net:  proto,
		}
		dns.HandleFunc(domain, handleRequest)
		g.Go(server.ListenAndServe)
		log.Println("INFO\tlisten", server.Net, server.Addr)
	}
	fmt.Println("")
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
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
					Ttl:    ttl,
				},
				A: req,
			})
		case dns.TypeNS:
			m.Answer = append(m.Answer, &dns.NS{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				Ns: domain,
			})
		case dns.TypeSOA:
			m.Answer = append(m.Answer, &dns.SOA{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				Ns:      domain,
				Mbox:    mbox,
				Serial:  serial,
				Refresh: refresh,
				Retry:   retry,
				Expire:  expire,
				Minttl:  minttl,
			})
		default:
			log.Printf("ERROR[%04x]\tunsupported qtype: %s\n", r.MsgHdr.Id, dns.TypeToString[q.Qtype])
		}

	}
	if err := w.WriteMsg(m); err != nil {
		log.Printf("ERROR[%04x]\t %v", r.MsgHdr.Id, err)
	}
}

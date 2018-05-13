package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	version  string
	revision string
	builtAt  string

	printVersion bool
	addr         string
	port         string
	protocols    = stringList{"tcp", "udp"}
	domains      = stringList{"."}
	ttl          = uint64(60)
	nameservers  stringList
	mbox         string
	serial       uint64
	refresh      = uint64(3600)
	retry        = uint64(900)
	expire       = uint64(604800)
	minttl       = uint64(3600)
	adminUser    = "admin"
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
	flag.Var(&nameservers, "ns", "nameserver list")
	flag.StringVar(&mbox, "mbox", "", "SOA mbox (default "+adminUser+"@<domain>)")
	flag.Uint64Var(&serial, "serial", serial, "SOA serial")
	flag.Uint64Var(&refresh, "refresh", refresh, "SOA refresh")
	flag.Uint64Var(&expire, "expire", expire, "SOA expire")
	flag.Uint64Var(&minttl, "minttl", minttl, "SOA minttl")
	flag.BoolVar(&printVersion, "v", false, "print version information")
}

func main() {
	flag.Parse()

	if printVersion {
		t, _ := strconv.ParseInt(builtAt, 10, 64)
		fmt.Printf("ipdns v%s (revision=%s, built_at=%s)\n", version, revision, time.Unix(t, 0).Format(time.RFC3339))
		return
	}

	// set defaut serial
	if serial == 0 {
		var err error
		serial, err = strconv.ParseUint(builtAt, 10, 64)
		if err != nil {
			errors.Wrap(err, "failed to parse variable: builtAt")
		}
	}

	g := errgroup.Group{}
	for _, proto := range protocols {
		for _, domain := range domains {
			if (domain == "" || domain == ".") && mbox == "" {
				log.Fatal("required to specify domain if mbox is blank")
			}

			server := &dns.Server{
				Addr: addr + ":" + port,
				Net:  proto,
			}
			dns.HandleFunc(domain, handleRequest(dns.Fqdn(domain)))
			g.Go(server.ListenAndServe)
			log.Printf("INFO\tlisten %s/%s\tdomain=%s\n", server.Addr, server.Net, domain)
		}
	}
	fmt.Println("")
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func handleRequest(domain string) dns.HandlerFunc {
	if domain == "." {
		domain = ""
	}
	mbox := strings.Replace(mbox, "@", ".", -1)
	if mbox == "" {
		mbox = adminUser + "." + domain
	}
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		for _, q := range r.Question {
			log.Printf("INFO [%04x]\trequest: remote=%v qtype=%v name=%v\n", r.MsgHdr.Id, w.RemoteAddr(), dns.TypeToString[q.Qtype], q.Name)
			if q.Qclass != dns.ClassINET {
				m.Rcode = dns.RcodeNotImplemented
				log.Printf("ERROR[%04x]\tunsupported qclass: %s\n", r.MsgHdr.Id, dns.ClassToString[q.Qclass])
				break
			}

			switch q.Qtype {
			case dns.TypeA:
				req := net.ParseIP(strings.TrimSuffix(q.Name, "."+domain)).To4()
				if req == nil || req.IsUnspecified() {
					m.Rcode = dns.RcodeNameError
					log.Printf("ERROR[%04x]\tinvalid domain name: %s\n", r.MsgHdr.Id, q.Name)
					break
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
				hdr := dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl),
				}
				if len(nameservers) == 0 {
					m.Answer = append(m.Answer, &dns.NS{Hdr: hdr, Ns: domain})
				} else {
					for _, ns := range nameservers {
						m.Answer = append(m.Answer, &dns.NS{Hdr: hdr, Ns: dns.Fqdn(ns)})
					}
				}
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
				m.Rcode = dns.RcodeNotImplemented
			}

		}
		if len(m.Answer) > 0 {
			m.Rcode = dns.RcodeSuccess
		}
		if err := w.WriteMsg(m); err != nil {
			log.Printf("ERROR[%04x]\t%v\n", r.MsgHdr.Id, err)
		}
	}
}

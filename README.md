# ipdns
Transform a domain name formatted by IPv4 to A record

## usage

```
Usage of bin/ipdns:
  -addr string
    	listen address
  -domain value
    	domain list (default .)
  -expire uint
    	SOA expire (default 604800)
  -mbox string
    	SOA mbox (default admin@<domain>)
  -minttl uint
    	SOA minttl (default 3600)
  -ns value
    	nameserver list
  -port string
    	listen port (default "53")
  -proto value
    	listen protocol list (default tcp,udp)
  -refresh uint
    	SOA refresh (default 3600)
  -serial uint
    	SOA serial
  -ttl uint
    	TTL (default 60)
  -v	print version information
```

## example

```bash
ipdns -domain ip.401.jp
```

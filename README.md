# droplet-lb

This is an IPv4 DNS server which exposes your Digital Ocean's inventory as
regular 'A' records.

## How does it work?

Give it a dns zone ( `droplet-lb` by default ) and query your instances via DNS.
Example:

This environment has 2 web servers ( `web-01` and `web-02` ), droplet-lb will
use the first part of the DNS name as a prefix during name lookup.

```
~ ❯❯❯ dig @localhost -p 8053 web.droplet-lb A +noall +answer

; <<>> DiG 9.8.3-P1 <<>> @localhost -p 8053 web.droplet-lb A +noall +answer
; (2 servers found)
;; global options: +cmd
web.droplet-lb.   30  IN  A 104.131.52.84
web.droplet-lb.   30  IN  A 159.203.175.202
```

A background task will refresh the list of instances every 60 seconds.

## Setup

Easiest way to set this up:

- Get an Ubuntu droplet
- Install nginx
- Download the linux binary from the releases page
- Get a [Personal Access Token](https://cloud.digitalocean.com/settings/api/tokens)
- Well.... run droplet-lb with your token and domain
- For a load-balancer setup, use the file [nginx.conf](nginx.conf) as starting point
- You can also forward a zone to unbound, dnsmasq and others

### Distributed setup

- Setup a dedicated instance for droplet-lb, binding to your private IP
- Setup multiple nginx instances, setting `resolver` to the proper droplet-lb
  instance
- Add Floating-IPs to the mix

This repo is actually a modified version of [DNS Toys](https://github.com/knadh/dns.toys) which was made to work with [ZeroLogDNS](https://zerologdns.com) infra.

# Some sample commands
In this version, you have to add ".zldns" after each "domain", which was needed to work with the ZeroLogDNS infrastructure.
```
dig budapest.time.zldns @127.0.0.10 -p 4053
dig budapest.weather.zldns @127.0.0.10 -p 4053
```
The full list can be found here: [www.dns.toys](https://www.dns.toys)

# Configuration

A sample configuration can be found here: [config.sample.toml](https://github.com/ZeroLogDNS/dnsfun/blob/main/config.sample.toml)

The configuration file should be mounted in `/app/config.toml` inside the docker.

# docker-compose.yml
```
version: '3.3'
services:
    dnsfun:
        ports:
            - 127.0.0.10:4053:4053/tcp
            - 127.0.0.10:4053:4053/udp
        image: darkstreet00/dnsfun:latest
        volumes:
            - ./config.toml:/app/config.toml
        restart: unless-stopped
```
# Hosted versions
If you don't want to host it yourself and just want to try it out, you can use the instance hosted by DNS Toys. (.zldns is not needed here.)
```
dig budapest.time @dns.toys
dig budapest.weather @dns.toys
```

If you want to use it in daily basis, you can use the ZeroLogDNS service, which is a standard DNS resolver that includes DNS Toys as well. (.zldns is needed here.)
```
kdig budapest.time.zldns +tls @dns.zerologdns.com
kdig budapest.weather.zldns +tls @dns.zerologdns.com
```
The ZeroLogDNS version is limited and does not have all the features that the DNS Toys version has.

For all available features.
```
kdig help.zldns +tls @dns.zerologdns.com
```
Output:
```
"convert numbers to words." "kdig 123456.words.zldns +tls @dns.zerologdns.com"
"convert numbers from one base to another" "kdig 100dec-hex.base.zldns +tls @dns.zerologdns.com"
"toss coin" "kdig coin.zldns +tls @dns.zerologdns.com"
"weather info" "kdig budapest.weather.zldns +tls @dns.zerologdns.com"
"get time for a city" "kdig budapest.time.zldns +tls @dns.zerologdns.com"
"convert currency rates" "kdig 100EUR-HUF.exchange.zldns +tls @dns.zerologdns.com"
"generate random numbers" "kdig 1-100.rand.zldns +tls @dns.zerologdns.com"
"convert cidr to ip range." "kdig 172.16.0.0/16.cidr.zldns +tls @dns.zerologdns.com"
```

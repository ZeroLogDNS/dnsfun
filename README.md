This repo is actually a modified version of [DNS Toys](https://github.com/knadh/dns.toys) which was made to work with [ZeroLogDNS](https://zerologdns.com) infra.

# Some sample commands
In this version, you have to add ".zldns" after each "domain", which was needed to work with the ZeroLogDNS infrastructure.
```
dig budapest.time.zldns @127.0.0.10
dig budapest.weather.zldns @127.0.0.10
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

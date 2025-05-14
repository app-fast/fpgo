## Fasthttp forward proxy

### Features

* Fasthttp
* http/https proxy
* ws/wss proxy
* Transparent Compression
* Multi DNS nameserves (-n "1.1.1.1,8.8.8.8")
* Graceful shutdown
* Battle-Tested and Production-Ready

### Usage

```
./fpgo -h # Show usage

Usage of ./fpgo:
  -a string
        Listen address. (default ":13002")
  -c int
        Max concurrency for fasthttp server (default 512)
  -h    Show usage
  -l int
        Log level. Examples: 0 (debug), 1 (info), 2 (warn), 3 (error). (default 1)
  -n string
        DNS nameserves, E.g. "8.8.8.8" or "1.1.1.1,8.8.8.8". Default is empty
  -t duration
        Connection timeout. Examples: 1m or 10s (default 1m0s)
  -v    Show version
```

### Example

```fish
./fpgo -a "0.0.0.0:13002" -c 1000 -n "8.8.8.8,1.1.1.1" -t 30s
curl -x http://localhost:13002 http(s)://example.com
```

### Caveats

This proxy server was used as a cheap knock-off of NAT Gateways originally, and was tuned for maximum performance, flexibility and less dependencies in our environments. But we've seen more and more people use it in production, so it's our responsibility to make sure everyone knows the following:

* Unlike other forward proxies like Squid, responses or files are not cached.
* Unlike other forward proxies, fpgo doesn't support user:password authentication, so beware of hackers taking over.
* A socks5 proxy is probably faster than a http proxy. If fpgo is fall short of expectations, take a look at socks5 implementations in go/rust. Nevertheless, not all http clients support socks5 forward proxy.

### Credits

* Original net/http implementation - https://www.sobyte.net/post/2021-09/https-proxy-in-golang-in-less-than-100-lines-of-code/
* Inspired by [goproxy](https://github.com/snail007/goproxy) - Closed-source (only very few lines of code are open-source) multi-purpose proxy. TBH this one seems to be unstable in heavy traffic that's why I made fpgo 💐💐

### Licence

Public domain

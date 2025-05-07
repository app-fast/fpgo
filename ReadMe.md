## Fasthttp forward proxy

### Features

* Fasthttp
* http/https proxy
* ws/wss proxy
* Compression
* Multi DNS nameserves (-n "1.1.1.1:53,8.8.8.8:53")
* Graceful shutdown
* battle-tested and production ready

### Usage

```
./fpgo -h # Show usage

Usage of ./fpgo:
  -a string
        Listen address. (default ":13002")
  -c int
        Max concurrency for fasthttp server (default 512)
  -h    Show usage
  -n string
        DNS nameserves, E.g. "8.8.8.8:53" or "1.1.1.1:53,8.8.8.8:53". Default is empty
  -t duration
        Connection timeout. Examples: 1m or 10s (default 1m0s)
  -v    Show version
```

### Example

```fish
./fpgo -a "0.0.0.0:13002" -c 1000 -n "8.8.8.8:53,1.1.1.1:53" -t 30s
curl -x http://localhost:13002 http(s)://example.com
```

### Credits

* Original net/http implementation - https://www.sobyte.net/post/2021-09/https-proxy-in-golang-in-less-than-100-lines-of-code/
* Inspired by [goproxy](https://github.com/snail007/goproxy) - Closed-source (only very few lines of code are open-source) multi-purpose proxy. TBH this one seems to be unstable in heavy traffic that's why I made fpgo 💐💐

### Licence

Public domain

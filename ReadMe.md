## Fasthttp forward proxy

### Features

* Fasthttp
* http/https proxy
* ws/wss proxy
* Transparent compression
* Multi DNS resolvers (-n "1.1.1.1:53,8.8.8.8:53")
* Graceful shutdown

### Usage

```sh
./fpgo -h # Show usage
./fpgo -a "0.0.0.0:13002" -c 1000 -n "8.8.8.8:53,1.1.1.1:53" -t 30s
curl -x http://localhost:13002 http(s)://example.com
```

### Credits

* net/http implementation - https://www.sobyte.net/post/2021-09/https-proxy-in-golang-in-less-than-100-lines-of-code/
* Inspired by [goproxy](https://github.com/snail007/goproxy) - Closed-source multi-purpose proxy. TBH this one seems to be unstable in heavy traffic that's why I made fpgo 💐💐

<h1 align="center">bs5 (better suo5)</h1>

<p align="center">一款高性能 HTTP 代理隧道工具</p>

<div align="center">

<b>本项目为suo5二开，旨在用标准项目代码实现suo5的基本功能，并去除suo5的流量特征</b>

</div>

----

`bs5` 是一个高性能 HTTP 隧道代理工具，它基于双向的 `Chunked-Encoding`
构建, 相比 [Neo-reGeorg](https://github.com/L-codes/Neo-reGeorg) 等传统隧道工具, `bs5`
的性能可以达到其数十倍。

相比于原版的改进：
- 移除了urfave/cli，改为使用Cobra（相比原版有部分命令行参数不同）
- 使用Viper，增加了多种配置文件支持
- 项目结构管理更清晰，将部分外部依赖整合到internal软件包中
- 重命名包名称，重构包结构
- 使用Makefile清晰明的配置文件，方便构建

其主要特性如下：

- 同时支持全双工与半双工模式，传输性能接近 FRP
- 支持在 Nginx 反向代理和负载均衡场景使用
- 支持 Java4 ~ Java 21 全版本和各大主流中间件服务
- 支持 IIS .Net Framework >= 2.0 的所有版本
- 完善的连接控制和并发管理，使用流畅丝滑
- 同时提供提供命令行和图形化界面

原理介绍 [https://koalr.me/posts/bs5-a-hign-performace-http-socks/](https://koalr.me/posts/bs5-a-hign-performace-http-socks/)

> 免责声明：此工具仅限于安全研究，用户承担因使用此工具而导致的所有法律和相关责任！作者不承担任何法律责任！

## 运行

```text
Usage:
  bs5 [flags]

Flags:
      --auth string                  socks5 creds, username:password, leave empty to auto generate
      --buf-size int                 request max body size (default 327680)
  -c, --config string                the filepath for json config file
  -d, --debug                        debug the traffic, print more details
  -E, --exclude-domain strings       exclude certain domain name for proxy, ex -E 'portswigger.net'
      --exclude-domain-file string   exclude certain domains for proxy in a file, one domain per line
  -f, --forward string               forward target address, enable forward mode when specified
  -H, --header strings               use extra header, ex -H 'Cookie: abc'
  -h, --help                         help for bs5
  -j, --jar                          enable cookiejar
  -l, --listen string                listen address of socks5 server (default "127.0.0.1:1111")
  -m, --method string                http request method (default "POST")
      --mode string                  connection mode, choices are auto, full, half (default "auto")
      --no-auth                      disable socks5 authentication (default true)
      --no-gzip                      disable gzip compression, which will improve compatibility with some old servers
      --no-heartbeat                 disable heartbeat to the remote server which will send data every 5s
  -p, --proxy strings                set upstream proxy, support socks5/http(s), eg: socks5://127.0.0.1:7890
  -r, --redirect string              redirect to the url if host not matched, used to bypass load balance
  -t, --target string                the remote server url, ex: http://localhost:8080/suo5.jsp
  -T, --test-exit string             test a real connection, if success exit(0), else exit(1)
      --timeout int                  request timeout in seconds (default 10)
      --ua string                    set the request User-Agent (default "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.1.2.3")
  -v, --version                      version for suo5
```

```bash
$ ./bs5 -t https://example.com/proxy.jsp
```

使用 `GET` 方法发送请求，有时可以绕过限制

```bash
$ ./bs5 -m GET -t https://example.com/proxy.jsp
```

自定义 socks5 监听在 `0.0.0.0:7788`，并自定义认证信息为 `test:test123`

```bash
$ ./bs5 -t https://example.com/proxy.jsp -l 0.0.0.0:7788 --auth test:test123
```

负载均衡场景下将流量转发到某一个固定的 url 解决请求被分散的问题，需要尽可能的在每一个后端服务中上传 bs5。
它的原理是判断 `-r` 中 URL 的 IP 是否与服务器的网卡 IP 匹配，不匹配则转发。

```bash
$ ./bs5 -t https://example.com/proxy.jsp -r http://172.0.3.2/code/proxy.jsp
```

配置域名/IP过滤规则，避免无意义的域名被代理, 命中规则的连接会直接被 reset 掉

```bash
# example.com 和 google.com 这两个域名不走代理
$ ./bs5 -t https://example.com/proxy.jsp -E example.com -E google.com

# 也可以将域名列表放在文件里，一行一个
$ ./bs5 -t https://example.com/proxy.jsp -ef ./excludes.txt

# 注意: 如果你配置的是域名，你需要确保 bs5 代理拿到的是域名，而不是解析好的 ip, 否则不会生效, 例如:
# 已经解析成 IP:  curl -v -x 'socks5://127.0.0.1:1111' https://example.com
# 仍然是域名:  curl -v -x 'socks5h://127.0.0.1:1111' https://example.com
```

### 特别提醒

`User-Agent` (`ua`) 的配置本地端与服务端是绑定的，如果修改了其中一个，另一个也必须对应修改才能连接上, 你可以将这个作为连接密码使用。

## 配置文件

配置文件的定义来自 `ctrl.bs5Config`, 完整的配置如下:

```json
{
  "method": "POST",
  "listen": "127.0.0.1:1111",
  "target": "",
  "no_auth": true,
  "username": "",
  "password": "",
  "mode": "auto",
  "buffer_size": 327680,
  "timeout": 10,
  "debug": false,
  "upstream_proxy": "",
  "redirect_url": "",
  "raw_header": [
    "User-Agent: Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.1.2.3"
  ],
  "disable_heartbeat": false,
  "disable_gzip": false,
  "disable_cookiejar": true,
  "exclude_domain": null
}
```


## 常见问题

1. 什么是全双工和半双工?

   **全双工** 仅需发送一个 HTTP 请求即可构建出一个 HTTP 隧道, 实现 TCP 的双向通信。可以理解成这个请求既是一个上传请求又是一个下载请求，只要连接不断开
   ，就会一直下载，一直上传, 便可以借此做双向通信。

   **半双工** 在部分场景下不支持 `全双工` 模式（比如有反代），可以退而求其次做半双工，即发送一个请求构建一个下行的隧道，同时用短链接发送上行数据一次来完成双向通信。

2. `bs5` 和 `Neo-reGeorg` 怎么选？

   如果目标是 Java 的站点，可以使用 `bs5` 来构建 http 隧道，大多数情况下 `bs5` 都要比 `neo` 更稳定速度更快。但 `neo`
   提供了非常多种类的服务端支持，兼容性很好，而且也支持一些 `bs5` 当前还在开发的功能，也支持更灵活的定制化。

## 接下来

- [ ] 修正配置文件支持
- [ ] 代码清理
- [ ] 添加容器测试，方便开发
- [ ] 流量特征去除
- [ ] 新增组网功能

## 原suo5项目
再次感谢原作者这款极其优秀的工具
- [https://github.com/zema1/suo5](https://github.com/zema1/suo5)
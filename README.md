# ZeroThrift RPC(Go)
## RPC的分层
ZeroThrift RPC分为4层，从前端往后端依次标记为L1, L2, L3, L4
### L1层(应用层)
* 最前端的RPC Client, 主要由thrift中间语言生成代码
* 将Thrift接口调用的数据经过适配，然后交给zeromq传输层
* zeromq传输层负责消息的传输，以及超时的处理，过期的message处理
* 上面思路: 我们修改了transport&protocol两个模块，通过简单的修改得到了python版本的zerothrift
	* Java, Go等也能很方面地实现自己的RPC Client
	* 其他语言只要支持Thrift和ZeroMq, 都可以方便地实现自己的Client

```python
# Transport层是所有的RPC服务都共用的(除非在同一个请求内部实现了并发)
_ = get_transport(settings.RPC_LOCAL_PROXY)

# 获取Client
from cy_typo.services.TypoService import Client
protocol = get_protocol("typo")
_typo_client = Client(protocol)

# 函数调用(需要注意: 所有的字符串都必须是utf8格式的，RPC在序列化string时会自动转换成为utf8)
rpc_result = _typo_client.correct_typo(content)
content = rpc_result.fixed
```
* 相应的Python RPC Client&Server的实现: https://github.com/wfxiang08/zerothrift

### L2层(Proxy层)
* 由于Python的进程功能太弱，不变在内部实现连接池等；如果让它们直接直连后端的Server, 则整个逻辑(connections)会非常乱，端口管理也会非常麻烦
* 服务的发现和负载均衡放在Client端赖做也会极大地增加了Client端的开发难度和负担
* Local Proxy层以产品为单位，负责发现所有的服务，并和L1层交互，让L1层不用关心服务部署的路径
* Proxy层也有简单的负载均衡的处理

```bash
# Go中的deamon似乎不太容易实现，借助: nohup &可以实现类似的效果(Codis也如此)
# 默认的proxy(绑定: 127.0.0.0:5550)
nohup rpc_proxy -c config.ini -L proxy.log >/dev/null 2>&1 &
# 在测试服务器上部署，给大家开发测试使用的proxy
nohup rpc_proxy -c config_test.ini -L proxy_test.log >/dev/null 2>&1 &
```

### L3层(负载均衡)
* Java/Go等天然支持多线程等，基本上不需要负载均衡, 因此这一层主要面向python
* 负责管理后端的Server, 自动进行负载均衡，以及处理后端服务的关闭，重启等异常情况
* 负责服务的注册
* 如果服务的正常关闭，会提前通知L2层，让L2层控制流量不再进入L3层的当前节点
* 如果服务异常关闭，则5s左右, L2层就会感知，并且下线对应的节点

```bash
## 配置文件
config.ini
zk=rd1:2181,rd2:2181,mysql2:2181

# 线上的product一律以: online开头
# 而测试的product禁止使用 online的服务
product=test
verbose=0

zk_session_timeout=30

## Load Balance
service=typo
front_host=
front_port=5555
back_address=tcp://127.0.0.1:5556

# 使用网络的IP, 如果没有指定front_host, 则使用使用当前机器的内网的Ip来注册
ip_prefix=10.

## Server
worker_pool_size=2

## Client/Proxy
proxy_address=tcp://127.0.0.1:5550


# Go中的deamon似乎不太容易实现，借助: nohup &可以实现类似的效果(Codis也如此)
nohup rpc_lb -c config.ini -L lb.log >/dev/null 2>&1 &
```

### L4层
* 对于Java/Go等，只需要从zeromq中读取task, 然后再返回处理的结果即可
	* 可以和L3组合，避免再自己去处理服务的注册等逻辑
* 对于Python, 直接使用zerothrift python框架即可
	* 通过thrift idl生成接口, Processor等
	* 实现Processor的接口
	* 合理地配置参数
		* 例如: worker_pool_size：如果是Django DB App则worker_pool_size=1(不支持异步数据库，多并发没有意义，即便支持异步数据库，Django的数据库逻辑也不支持高并发); 但是如果不使用DB, 那么Django还是可以支持多并发的
		* 注意在iptables中开启相关的端口
    * thrift只支持utf8格式的字符串，因此在使用过程中注意
    	* 字符串如果是unicode, 一方面len可能和utf8的 len不一样，另一方面，数据在序列化时可能报编码错误

```python
class TypoProcessor(object):
    def correct_typo(self, query):
        # print "Query: ", query
        result = TypoResult()
        new_query, mappings = typo_manager.correct_typo(query)
        result.fixed = ensure_utf8(new_query)
        result.fixes = []
        for old, new in mappings:
			# 注意字符串的编码: utf8
            result.fixes.append(FixedTerm(ensure_utf8(old), ensure_utf8(new)))
        return result

config_path = "config.ini"
config = parse_config(config_path)
endpoint = config["back_address"]
service = config["service"]
worker_pool_size = int(config["worker_pool_size"])

processor = Processor(TypoProcessor())
s = Server(processor, pool_size=worker_pool_size, mode_ppworker = True, service=service)
s.connect(endpoint)
s.run()
```

## 运维部署
* 编译:
	* go build github.com/wfxiang08/rpc_proxy/demo/rpc_lb.go
	* go build github.com/wfxiang08/rpc_proxy/demo/rpc_proxy.go
	* scp rpc_* node:/usr/local/bin/
	* sudo cp rpc_* /usr/local/bin/

* zeromq的安装部署
	* https://github.com/wfxiang08/rpc_proxy/blob/master/INSTALL.md

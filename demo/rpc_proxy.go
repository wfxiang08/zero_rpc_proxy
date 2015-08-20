package main

import (
	"github.com/docopt/docopt-go"
	color "github.com/fatih/color"
	zmq "github.com/pebbe/zmq4"
	config "github.com/wfxiang08/rpc_proxy/config"
	proxy "github.com/wfxiang08/rpc_proxy/proxy"
	utils "github.com/wfxiang08/rpc_proxy/utils"
	"github.com/wfxiang08/rpc_proxy/utils/bytesize"
	"github.com/wfxiang08/rpc_proxy/utils/log"
	zk "github.com/wfxiang08/rpc_proxy/zk"
	"os"
	"strings"
	"syscall"
	"time"
)

const (
	PROXY_FRONT_END    = "rpc_front"
	HEARTBEAT_INTERVAL = 1000 * time.Millisecond //  msecs
)

var magenta = color.New(color.FgMagenta).SprintFunc()

var usage = `usage: rpc_proxy [-c <config_file>] [--product=<product-name>]   [--zk=<zookeeper-address>] [--faddr=<frontend-address>] [-L <log_file>] [--log-level=<loglevel>] [--log-filesize=<filesize>] 

options:
   -c <config_file>
   --product=<product-name> eg. online_medical
   --faddr=<frontend-address> backend address: tcp://*:5555
   --zk=<zookeeper-address> frontend address: tcp://*:5556
   -L	set output log file, default is stdout
   --log-level=<loglevel>	set log level: info, warn, error, debug [default: info]
   --log-filesize=<maxsize>  set max log file size, suffixes "KB", "MB", "GB" are allowed, 1KB=1024 bytes, etc. Default is 1GB.
`

//
// Proxy关闭，则整个机器就OVER, 需要考虑将整个机器下线
// 因此Proxy需要设计的非常完美，不要轻易地被杀死，或自杀
//
func main() {
	// 解析输入参数
	args, err := docopt.Parse(usage, nil, true, "Chunyu RPC Local Proxy v0.1", true)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	var maxFileFrag = 10
	var maxFragSize int64 = bytesize.GB * 1
	if s, ok := args["--log-filesize"].(string); ok && s != "" {
		v, err := bytesize.Parse(s)
		if err != nil {
			log.PanicErrorf(err, "invalid max log file size = %s", s)
		}
		maxFragSize = v
	}

	// set output log file
	if s, ok := args["-L"].(string); ok && s != "" {
		f, err := log.NewRollingFile(s, maxFileFrag, maxFragSize)
		if err != nil {
			log.PanicErrorf(err, "open rolling log file failed: %s", s)
		} else {
			defer f.Close()
			log.StdLog = log.New(f, "")
		}
	}
	log.SetLevel(log.LEVEL_INFO)
	log.SetFlags(log.Flags() | log.Lshortfile)

	// set log level
	if s, ok := args["--log-level"].(string); ok && s != "" {
		setLogLevel(s)
	}

	var zkAddr, frontAddr, productName string

	// 从config文件中读取数据
	if args["-c"] != nil {
		configFile := args["-c"].(string)
		conf, err := utils.LoadConf(configFile)
		if err != nil {
			log.PanicErrorf(err, "load config failed")
		}
		productName = conf.ProductName
		frontAddr = conf.ProxyAddr
		zkAddr = conf.ZkAddr
		config.VERBOSE = conf.Verbose
	} else {
		productName = ""
		zkAddr = ""
	}

	if s, ok := args["--product"].(string); ok && s != "" {
		productName = s
	} else if productName == "" {
		log.PanicErrorf(err, "Invalid ProductName: %s", s)
	}

	if s, ok := args["--zk"].(string); ok && s != "" {
		zkAddr = s
	} else if zkAddr == "" {
		log.PanicErrorf(err, "Invalid zookeeper address: %s", s)
	}

	if s, ok := args["--faddr"].(string); ok && s != "" {
		frontAddr = s
	} else if frontAddr == "" {
		log.PanicErrorf(err, "Invalid Proxy address: %s", s)
	}

	// 正式的服务
	mainBody(productName, frontAddr, zkAddr)
}

//
// 两参数是必须的:  ProductName, zkAddress, frontAddr可以用来测试
//
func mainBody(productName string, frontAddr string, zkAdresses string) {
	// 1. 创建到zk的连接
	var topo *zk.Topology
	topo = zk.NewTopology(productName, zkAdresses)

	// 3. 读取后端服务的配置
	poller := zmq.NewPoller()
	backServices := proxy.NewBackServices(poller, productName, topo)

	// 4. 创建前端服务
	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	defer frontend.Close()

	// ROUTER/ROUTER绑定到指定的端口
	log.Println("---->Bind: ", magenta(frontAddr))
	frontend.Bind(frontAddr) //  For clients

	// 开始监听前端服务
	poller.Add(frontend, zmq.POLLIN)

	for {
		var sockets []zmq.Polled
		var err error

		sockets, err = poller.Poll(HEARTBEAT_INTERVAL)

		if err != nil {
			log.Println("Encounter Errors, Services Stoped: ", err)
			continue
		}

		for _, socket := range sockets {
			switch socket.Socket {

			case frontend:
				if config.VERBOSE {
					log.Println("----->Message from front: ")
				}
				msgs, err := frontend.RecvMessage(0)

				if err != nil {
					continue //  Interrupted
				}
				var service string
				var client_id string

				utils.PrintZeromqMsgs(msgs, "ProxyFrontEnd")

				// msg格式: <client_id, '', service,  '', other_msgs>
				client_id, msgs = utils.Unwrap(msgs)
				service, msgs = utils.Unwrap(msgs)

				//				log.Println("Client_id: ", client_id, ", Service: ", service)

				backService := backServices.GetBackService(service)

				if backService == nil {
					log.Println("BackService Not Found...")
					// 最后一个msg为Thrift编码后的消息
					thriftMsg := msgs[len(msgs)-1]
					// XXX: seqId如果不需要，也可以使用固定的数字
					_, _, seqId, _ := proxy.ParseThriftMsgBegin([]byte(thriftMsg))
					errMsg := proxy.GetServiceNotFoundData(service, seqId)

					// <client_id, "", errMsg>
					if len(msgs) > 1 {
						frontend.SendMessage(client_id, "", msgs[0:len(msgs)-1], errMsg)
					} else {
						frontend.SendMessage(client_id, "", errMsg)
					}

				} else {
					// <"", client_id, "", msgs>
					total, err, errMsg := backService.HandleRequest(client_id, msgs)
					if errMsg != nil {
						if config.VERBOSE {
							log.Println("backService Error for service: ", service)
						}
						if len(msgs) > 1 {
							frontend.SendMessage(client_id, "", msgs[0:len(msgs)-1], *errMsg)
						} else {
							frontend.SendMessage(client_id, "", *errMsg)
						}
					} else if err != nil {
						log.Println(utils.Red("backService.HandleRequest Error: "), err, ", Total: ", total)
					}
				}
			default:
				// 除了来自前端的数据，其他的都来自后端
				msgs, err := socket.Socket.RecvMessage(0)
				if err != nil {
					log.Println("Encounter Errors When receiving from background")
					continue //  Interrupted
				}
				if config.VERBOSE {
					utils.PrintZeromqMsgs(msgs, "proxy")
				}

				msgs = utils.TrimLeftEmptyMsg(msgs)

				// msgs格式: <client_id, "", rpc_data>
				//          <control_msg_rpc_data>
				if len(msgs) == 1 {
					// 告知后端的服务可能有问题

				} else {
					frontend.SendMessage(msgs)
				}
			}
		}
	}
}

func init() {
	log.SetLevel(log.LEVEL_INFO)
}

func setLogLevel(level string) {
	var lv = log.LEVEL_INFO

	switch strings.ToLower(level) {
	case "error":
		lv = log.LEVEL_ERROR
	case "warn", "warning":
		lv = log.LEVEL_WARN
	case "debug":
		lv = log.LEVEL_DEBUG
	case "info":
		fallthrough
	default:
		lv = log.LEVEL_INFO
	}
	log.SetLevel(lv)
	log.Infof("set log level to %s", lv)
}

func setCrashLog(file string) {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.InfoErrorf(err, "cannot open crash log file: %s", file)
	} else {
		syscall.Dup2(int(f.Fd()), 2)
	}
}

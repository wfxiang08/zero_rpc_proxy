//
//  Paranoid Pirate queue. 参考: http://zguide.zeromq.org/php:chapter4
//
package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
	color "github.com/fatih/color"
	zmq "github.com/pebbe/zmq4"
	topozk "github.com/wandoulabs/go-zookeeper/zk"
	config "github.com/wfxiang08/rpc_proxy/config"
	proxy "github.com/wfxiang08/rpc_proxy/proxy"
	queue "github.com/wfxiang08/rpc_proxy/queue"
	utils "github.com/wfxiang08/rpc_proxy/utils"
	"github.com/wfxiang08/rpc_proxy/utils/bytesize"
	"github.com/wfxiang08/rpc_proxy/utils/log"
	zk "github.com/wfxiang08/rpc_proxy/zk"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	HEARTBEAT_LIVENESS = 3                       //  3-5 is reasonable
	HEARTBEAT_INTERVAL = 1000 * time.Millisecond //  msecs

	PPP_READY         = uint8('\x01') // 通知lb, Worker Ready
	PPP_HEARTBEAT     = uint8('\x02') // 通知lb, Worker 还活着
	PPP_HEARTBEAT_STR = "\x02"
	PPP_STOP          = uint8('\x03') // 通知lb, Worker 即将关闭，如果有什么Event请不要再分配了

	VERSION = "\x01" //  当前协议的版本
)

var magenta = color.New(color.FgMagenta).SprintFunc()

var usage = `usage: rpc_lb [-c <config_file>] [--product=<product-name>]  [--zk=<zookeeper-address>] [--service=<service-name>] [--faddr=<frontend-address>] [--baddr=<backend-address>] [-L <log_file>] [--log-level=<loglevel>] [--log-filesize=<filesize>] 

options:
   -c <config_file>
   --zk=<zookeeper-address>
   --product=<product-name>
   --service=<service-name>
   --faddr=<backend-address> backend address: tcp://127.0.0.1:5555
   --baddr=<frontend-address> frontend address: tcp://127.0.0.1:5556
   -L	set output log file, default is stdout
   --log-level=<loglevel>	set log level: info, warn, error, debug [default: info]
   --log-filesize=<maxsize>  set max log file size, suffixes "KB", "MB", "GB" are allowed, 1KB=1024 bytes, etc. Default is 1GB.
`

//
// Load Balance如何运维呢?
// 1. 在服务提供方，会会启动Load Balance, 它只负责本机器的某个指定服务的lb
// 2. 正常情况下，不能被轻易杀死
// 3. 需要考虑 graceful stop, 在死之前告知所有的proxy，如何告知呢? TODO
//
//
func main() {
	args, err := docopt.Parse(usage, nil, true, "Chunyu RPC Load Balance v0.1", true)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	var maxFileFrag = 2
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
	var backendAddr, frontendAddr, zkAddr, productName, serviceName string

	// set config file
	if args["-c"] != nil {
		configFile := args["-c"].(string)
		conf, err := utils.LoadConf(configFile)
		if err != nil {
			log.PanicErrorf(err, "load config failed")
		}
		productName = conf.ProductName

		if conf.FrontHost == "" {
			fmt.Println("FrontHost: ", conf.FrontHost, ", Prefix: ", conf.IpPrefix)
			if conf.IpPrefix != "" {
				conf.FrontHost = utils.GetIpWithPrefix(conf.IpPrefix)
			}
		}
		if conf.FrontPort != "" && conf.FrontHost != "" {
			frontendAddr = fmt.Sprintf("tcp://%s:%s", conf.FrontHost, conf.FrontPort)
		}

		backendAddr = conf.BackAddr
		serviceName = conf.Service

		zkAddr = conf.ZkAddr
		config.VERBOSE = conf.Verbose

	} else {
		productName = ""
		zkAddr = ""
	}

	if s, ok := args["--product"].(string); ok && s != "" {
		productName = s
	} else if productName == "" {
		// 既没有config指定，也没有命令行指定，则报错
		log.PanicErrorf(err, "Invalid ProductName: %s", s)
	}

	if s, ok := args["--zk"].(string); ok && s != "" {
		zkAddr = s
	} else if zkAddr == "" {
		log.PanicErrorf(err, "Invalid zookeeper address: %s", s)
	}

	if s, ok := args["--service"].(string); ok && s != "" {
		serviceName = s
	} else if serviceName == "" {
		log.PanicErrorf(err, "Invalid ServiceName: %s", s)
	}

	if s, ok := args["--baddr"].(string); ok && s != "" {
		backendAddr = s
	} else if backendAddr == "" {
		log.PanicErrorf(err, "Invalid backend address: %s", s)
	}
	if s, ok := args["--faddr"].(string); ok && s != "" {
		frontendAddr = s
	} else if frontendAddr == "" {
		//
		log.PanicErrorf(err, "Invalid frontend address: %s", s)
	}

	// 正式的服务
	mainBody(zkAddr, productName, serviceName, frontendAddr, backendAddr)
}

// tcp://127.0.0.1:5555 --> tcp://127_0_0_1:5555
func GetServiceIdentity(frontendAddr string) string {
	fid := strings.Replace(frontendAddr, ".", "_", -1)
	fid = strings.Replace(fid, ":", "_", -1)
	fid = strings.Replace(fid, "//", "", -1)
	return fid
}

func mainBody(zkAddr string, productName string, serviceName string, frontendAddr string, backendAddr string) {
	// 1. 创建到zk的连接
	var topo *zk.Topology
	topo = zk.NewTopology(productName, zkAddr)

	// 2. 启动服务
	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	backend, _ := zmq.NewSocket(zmq.ROUTER)
	defer frontend.Close()
	defer backend.Close()

	// ROUTER/ROUTER绑定到指定的端口

	// tcp://127.0.0.1:5555 --> tcp://127_0_0_1:5555
	lbServiceName := GetServiceIdentity(frontendAddr)

	frontend.SetIdentity(lbServiceName)
	frontend.Bind(frontendAddr) //  For clients "tcp://*:5555"
	backend.Bind(backendAddr)   //  For workers "tcp://*:5556"

	log.Printf("FrontAddr: %s, BackendAddr: %s\n", magenta(frontendAddr), magenta(backendAddr))

	// 后端的workers queue
	workersQueue := queue.NewPPQueue()

	// 心跳间隔1s
	heartbeat_at := time.Tick(HEARTBEAT_INTERVAL)

	poller1 := zmq.NewPoller()
	poller1.Add(backend, zmq.POLLIN)

	poller2 := zmq.NewPoller()
	// 前提:
	//     1. 当zeromq通知消息可读时，那么整个Message(所有的msg parts)都可读
	//	   2. 往zeromq写数据时，是异步的，因此也不存在block(除非数据量巨大)
	//
	poller2.Add(backend, zmq.POLLIN)
	poller2.Add(frontend, zmq.POLLIN)

	// 3. 注册zk
	var endpointInfo map[string]interface{} = make(map[string]interface{})
	endpointInfo["frontend"] = frontendAddr
	endpointInfo["backend"] = backendAddr

	topo.AddServiceEndPoint(serviceName, lbServiceName, endpointInfo)

	isAlive := true
	isAliveLock := &sync.RWMutex{}

	go func() {
		servicePath := topo.ProductServicePath(serviceName)
		evtbus := make(chan interface{})
		for true {
			// 只是为了监控状态
			_, err := topo.WatchNode(servicePath, evtbus)

			if err == nil {
				// 等待事件
				e := (<-evtbus).(topozk.Event)
				if e.State == topozk.StateExpired || e.Type == topozk.EventNotWatching {
					// Session过期了，则需要删除之前的数据，因为这个数据的Owner不是当前的Session
					topo.DeleteServiceEndPoint(serviceName, lbServiceName)
					topo.AddServiceEndPoint(serviceName, lbServiceName, endpointInfo)
				}
			} else {
				time.Sleep(time.Second)
			}

			isAliveLock.RLock()
			isAlive1 := isAlive
			isAliveLock.RUnlock()
			if !isAlive1 {
				break
			}

		}
	}()

	ch := make(chan os.Signal, 1)

	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	// syscall.SIGKILL
	// kill -9 pid
	// kill -s SIGKILL pid 还是留给运维吧
	//

	// 自动退出条件:
	//

	var suideTime time.Time

	for {
		var sockets []zmq.Polled
		var err error

		sockets, err = poller2.Poll(HEARTBEAT_INTERVAL)
		if err != nil {
			//			break //  Interrupted
			log.Errorf("Error When Pollling: %v\n", err)
			continue
		}

		hasValidMsg := false
		for _, socket := range sockets {
			switch socket.Socket {
			case backend:
				// 格式:
				// 后端:
				// 	             <"", proxy_id, "", client_id, "", rpc_data>
				// Backend Socket读取到的:
				//		<wokerid, "", proxy_id, "", client_id, "", rpc_data>
				//
				msgs, err := backend.RecvMessage(0)
				if err != nil {
					log.Errorf("Error When RecvMessage from background: %v\n", err)
					continue
				}
				if config.VERBOSE {
					// log.Println("Message from backend: ", msgs)
				}
				// 消息类型:
				// msgs: <worker_id, "", proxy_id, "", client_id, "", rpc_data>
				//       <worker_id, "", rpc_control_data>
				worker_id, msgs := utils.Unwrap(msgs)

				// rpc_control_data 控制信息
				// msgs: <rpc_control_data>
				if len(msgs) == 1 {
					// PPP_READY
					// PPP_HEARTBEAT
					controlMsg := msgs[0]

					// 碰到无效的信息，则直接跳过去
					if len(controlMsg) == 0 {
						continue
					}
					if config.VERBOSE {
						// log.Println("Got Message From Backend...")
					}

					if controlMsg[0] == PPP_READY || controlMsg[0] == PPP_HEARTBEAT {
						// 后端服务剩余的并发能力
						var concurrency int
						if len(controlMsg) >= 3 {
							concurrency = int(controlMsg[2])
						} else {
							concurrency = 1
						}
						if config.VERBOSE {
							// utils.PrintZeromqMsgs(msgs, "control msg")
						}

						force_update := controlMsg[0] == PPP_READY
						workersQueue.UpdateWorkerStatus(worker_id, concurrency, force_update)
					} else if controlMsg[0] == PPP_STOP {
						// 停止指定的后端服务
						workersQueue.UpdateWorkerStatus(worker_id, -1, true)
					} else {
						log.Errorf("Unexpected Control Message: %d", controlMsg[0])
					}
				} else {
					hasValidMsg = true
					// 将信息发送到前段服务, 如果前端服务挂了，则消息就丢失
					//					log.Println("Send Message to frontend")
					workersQueue.UpdateWorkerStatus(worker_id, 0, false)
					// msgs: <proxy_id, "", client_id, "", rpc_data>
					frontend.SendMessage(msgs)
				}
			case frontend:
				hasValidMsg = true
				log.Println("----->Message from front: ")
				msgs, err := frontend.RecvMessage(0)
				if err != nil {
					log.Errorf("Error when reading from frontend: %v\n", err)
					continue
				}

				// msgs:
				// <proxy_id, "", client_id, "", rpc_data>
				if config.VERBOSE {
					utils.PrintZeromqMsgs(msgs, "frontend")
				}
				msgs = utils.TrimLeftEmptyMsg(msgs)

				// 将msgs交给后端服务器
				worker := workersQueue.NextWorker()
				if worker != nil {
					if config.VERBOSE {
						log.Println("Send Msg to Backend worker: ", worker.Identity)
					}
					backend.SendMessage(worker.Identity, "", msgs)
				} else {
					// 怎么返回错误消息呢?
					if config.VERBOSE {
						log.Println("No backend worker found")
					}
					errMsg := proxy.GetWorkerNotFoundData("account", 0)

					// <proxy_id, "", client_id, "", rpc_data>
					frontend.SendMessage(msgs[0:(len(msgs)-1)], errMsg)
				}
			}
		}

		// 如果安排的suiside, 则需要处理 suiside的时间
		isAliveLock.RLock()
		isAlive1 := isAlive
		isAliveLock.RUnlock()

		if !isAlive1 {
			if hasValidMsg {
				suideTime = time.Now().Add(time.Second * 3)
			} else {
				if time.Now().After(suideTime) {
					log.Println(utils.Green("Load Balance Suiside Gracefully"))
					break
				}
			}
		}

		// 心跳同步
		select {
		case <-heartbeat_at:
			now := time.Now()

			// 给workerQueue中的所有的worker发送心跳消息
			for _, worker := range workersQueue.WorkerQueue {
				if worker.Expire.After(now) {
					//					log.Println("Sending Hb to Worker: ", worker.Identity)
					backend.SendMessage(worker.Identity, "", PPP_HEARTBEAT_STR)
				}
			}

			workersQueue.PurgeExpired()
		case sig := <-ch:
			isAliveLock.Lock()
			isAlive1 := isAlive
			isAlive = false
			isAliveLock.Unlock()

			if isAlive1 {
				// 准备退出(但是需要处理完毕手上的活)

				// 需要退出:
				topo.DeleteServiceEndPoint(serviceName, lbServiceName)

				if sig == syscall.SIGKILL {
					log.Println(utils.Red("Got Kill Signal, Return Directly"))
					break
				} else {
					suideTime = time.Now().Add(time.Second * 3)
					log.Println(utils.Red("Schedule to suicide at: "), suideTime.Format("@2006-01-02 15:04:05"))
				}
			}
		default:
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

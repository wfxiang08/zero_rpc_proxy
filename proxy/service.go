package proxy

import (
	"fmt"
	zmq "github.com/pebbe/zmq4"
	config "github.com/wfxiang08/rpc_proxy/config"
	utils "github.com/wfxiang08/rpc_proxy/utils"
	"github.com/wfxiang08/rpc_proxy/utils/log"
	zk "github.com/wfxiang08/rpc_proxy/zk"
	"sync"
	"time"
)

type BackService struct {
	ServiceName string
	// 如何处理呢?
	// 可以使用zeromq本身的连接到多个endpoints的特性，自动负载均衡
	backend *BackSockets
	poller  *zmq.Poller
	topo    *zk.Topology
}

// 创建一个BackService
func NewBackService(serviceName string, poller *zmq.Poller, topo *zk.Topology) *BackService {

	backSockets := NewBackSockets(poller)

	service := &BackService{
		ServiceName: serviceName,
		backend:     backSockets,
		poller:      poller,
		topo:        topo,
	}

	var evtbus chan interface{} = make(chan interface{}, 2)
	servicePath := topo.ProductServicePath(serviceName)
	endpoints, err := topo.WatchChildren(servicePath, evtbus)
	if err != nil {
		log.Println("Error: ", err)
		panic("Reading Service List Failed: ")
	}

	go func() {
		for true {
			// 如何监听endpoints的变化呢?
			addrSet := make(map[string]bool)
			nowStr := time.Now().Format("@2006-01-02 15:04:05")
			for _, endpoint := range endpoints {
				// 这些endpoint变化该如何处理呢?
				log.Println(utils.Green("---->Find Endpoint: "), endpoint)
				endpointInfo, _ := topo.GetServiceEndPoint(serviceName, endpoint)

				addr, ok := endpointInfo["frontend"]
				if ok {
					addrStr := addr.(string)
					log.Println(utils.Green("---->Add endpoint to backend: "), addrStr, nowStr)
					addrSet[addrStr] = true
				}
			}

			service.backend.UpdateEndpointAddrs(addrSet)

			// 等待事件
			<-evtbus
			// 读取数据，继续监听
			endpoints, err = topo.WatchChildren(servicePath, evtbus)
		}
	}()

	ticker := time.NewTicker(time.Millisecond * 1000)
	go func() {
		for _ = range ticker.C {
			service.backend.PurgeEndpoints()
		}
	}()

	return service

}

//
// 将消息发送到Backend上去
//
func (s *BackService) HandleRequest(client_id string, msgs []string) (total int, err error, msg *[]byte) {

	backSocket := s.backend.NextSocket()
	if backSocket == nil {
		// 没有后端服务

		if config.VERBOSE {
			log.Println(utils.Red("No BackSocket Found"))
		}
		errMsg := GetWorkerNotFoundData(s.ServiceName, 0)
		return 0, nil, &errMsg
	} else {
		if config.VERBOSE {
			log.Println("SendMessage With: ", backSocket.Addr)
		}
		total, err = backSocket.SendMessage("", client_id, "", msgs)
		return total, err, nil
	}
}

// BackServices通过topology来和zk进行交互
type BackServices struct {
	sync.RWMutex
	Services map[string]*BackService

	// 在zk中标记下线的
	OfflineServices map[string]*BackService

	poller *zmq.Poller
	topo   *zk.Topology
}

func NewBackServices(poller *zmq.Poller, productName string, topo *zk.Topology) *BackServices {

	// 创建BackServices
	result := &BackServices{
		Services:        make(map[string]*BackService),
		OfflineServices: make(map[string]*BackService),
		poller:          poller,
		topo:            topo,
	}

	var evtbus chan interface{} = make(chan interface{}, 2)
	servicesPath := topo.ProductServicesPath()
	path, e1 := topo.CreateDir(servicesPath) // 保证Service目录存在，否则会报错
	fmt.Println("Path: ", path, "error: ", e1)
	services, err := topo.WatchChildren(servicesPath, evtbus)
	if err != nil {
		log.Println("Error: ", err)
		panic("Reading Service List Failed")
	}

	go func() {
		for true {

			result.Lock()
			for _, service := range services {
				log.Println("Service: ", service)
				if _, ok := result.Services[service]; !ok {
					result.addBackService(service)
				}
			}
			result.Unlock()

			// 等待事件
			<-evtbus
			// 读取数据，继续监听(连接过期了就过期了，再次Watch即可)
			services, err = topo.WatchChildren(servicesPath, evtbus)
		}
	}()

	// 读取zk, 等待
	log.Println("ProductName: ", result.topo.ProductName)

	return result
}

// 添加一个后台服务
func (bk *BackServices) addBackService(service string) {

	backService, ok := bk.Services[service]
	if !ok {
		backService = NewBackService(service, bk.poller, bk.topo)
		bk.Services[service] = backService
	}

}
func (bk *BackServices) GetBackService(service string) *BackService {
	bk.RLock()
	backService, ok := bk.Services[service]
	bk.RUnlock()

	if ok {
		return backService
	} else {
		return nil
	}
}

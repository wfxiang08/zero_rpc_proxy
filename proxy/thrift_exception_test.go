package proxy

import (
	"fmt"
	thrift "git.apache.org/thrift.git/lib/go/thrift"
	"github.com/wfxiang08/rpc_proxy/utils/assert"
	"strings"
	"testing"
)

func TestGetThriftException(t *testing.T) {

	//	serviceName := "accounts"
	//	data := GetServiceNotFoundData(serviceName, 0)
	//	fmt.Println("Exception Data: ", data)

	//	transport := thrift.NewTMemoryBufferLen(1024)
	//	transport.Write(data)
	//	//	transport.Flush()

	//	exc := thrift.NewTApplicationException(-1, "")
	//	protocol := thrift.NewTBinaryProtocolTransport(transport)

	//	// 注意: Read函数返回的是一个新的对象
	//	exc, _ = exc.Read(protocol)

	//	fmt.Println("Exc: ", exc.TypeId(), "Error: ", exc.Error())

	//	var errMsg string = exc.Error()
	//	assert.Must(strings.Contains(errMsg, serviceName))
}

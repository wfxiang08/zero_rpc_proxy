package proxy

import (
	"fmt"
	thrift "git.apache.org/thrift.git/lib/go/thrift"
)

//
// 生成Thrift格式的Exception Message
//
func GetServiceNotFoundData(service string, seqId int32) []byte {
	// 构建thrift的Transport
	transport := thrift.NewTMemoryBufferLen(1024)
	protocol := thrift.NewTBinaryProtocolTransport(transport)

	// 构建一个Message, 写入Exception
	msg := fmt.Sprintf("Service: %s Not Found", service)
	exc := thrift.NewTApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, msg)

	protocol.WriteMessageBegin(service, thrift.EXCEPTION, seqId)
	exc.Write(protocol)
	protocol.WriteMessageEnd()

	bytes := transport.Bytes()
	return bytes
}

func GetWorkerNotFoundData(service string, seqId int32) []byte {
	// 构建thrift的Transport
	transport := thrift.NewTMemoryBufferLen(1024)
	protocol := thrift.NewTBinaryProtocolTransport(transport)

	// 构建一个Message, 写入Exception
	msg := fmt.Sprintf("Worker: %s Not Found", service)
	exc := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, msg)

	protocol.WriteMessageBegin(service, thrift.EXCEPTION, seqId)
	exc.Write(protocol)
	protocol.WriteMessageEnd()

	bytes := transport.Bytes()
	return bytes
}

//
// 解析Thrift数据的Message Header
//
func ParseThriftMsgBegin(msg []byte) (name string, typeId thrift.TMessageType, seqId int32, err error) {
	transport := thrift.NewTMemoryBufferLen(1024)
	transport.Write(msg)
	protocol := thrift.NewTBinaryProtocolTransport(transport)
	name, typeId, seqId, err = protocol.ReadMessageBegin()
	return
}

// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/c4pt0r/cfg"
	color "github.com/fatih/color"
	"github.com/wandoulabs/zkhelper"
	"github.com/wfxiang08/rpc_proxy/utils/errors"
	"github.com/wfxiang08/rpc_proxy/utils/log"
)

var Red = color.New(color.FgRed).SprintFunc()
var Green = color.New(color.FgGreen).SprintFunc()

func InitConfig() (*cfg.Cfg, error) {
	configFile := os.Getenv("CODIS_CONF")
	if len(configFile) == 0 {
		configFile = "config.ini"
	}
	ret := cfg.NewCfg(configFile)
	if err := ret.Load(); err != nil {
		return nil, errors.Trace(err)
	} else {
		return ret, nil
	}
}

func InitConfigFromFile(filename string) (*cfg.Cfg, error) {
	ret := cfg.NewCfg(filename)
	if err := ret.Load(); err != nil {
		return nil, errors.Trace(err)
	}
	return ret, nil
}

//
// 获取带有指定Prefix的Ip
//
func GetIpWithPrefix(prefix string) string {

	ifaces, _ := net.Interfaces()
	// handle err
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			ipAddr := ip.String()
			//			fmt.Println("ipAddr: ", ipAddr)
			if strings.HasPrefix(ipAddr, prefix) {
				return ipAddr
			}

		}
	}
	return ""
}

func GetZkLock(zkConn zkhelper.Conn, productName string) zkhelper.ZLocker {
	zkPath := fmt.Sprintf("/zk/codis/db_%s/LOCK", productName)
	return zkhelper.CreateMutex(zkConn, zkPath)
}

func GetExecutorPath() string {
	filedirectory := filepath.Dir(os.Args[0])
	execPath, err := filepath.Abs(filedirectory)
	if err != nil {
		log.PanicErrorf(err, "get executor path failed")
	}
	return execPath
}

type Strings []string

func (s1 Strings) Eq(s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

const (
	EMPTY_MSG = ""
)

//
// <head> "", tail... ----> head, tail...
// 将msgs拆分成为两部分, 第一部分为: head(包含路由信息);第二部分为: tails包含信息部分
//
func Unwrap(msgs []string) (head string, tails []string) {
	head = msgs[0]
	if len(msgs) > 1 && msgs[1] == EMPTY_MSG {
		tails = msgs[2:]
	} else {
		tails = msgs[1:]
	}
	return
}

// 将msgs中前面多余的EMPTY_MSG删除，可能是 zeromq的不同的socket的配置不匹配导致的
func TrimLeftEmptyMsg(msgs []string) []string {
	for index, msg := range msgs {
		if msg != EMPTY_MSG {
			return msgs[index:len(msgs)]
		}
	}
	return msgs
}

// 打印zeromq中的消息，用于Debug
func PrintZeromqMsgs(msgs []string, prefix string) {

	//	fmt.Printf("Message Length: %d, Prefix: %s\n", len(msgs), prefix)
	//	for idx, msg := range msgs {
	//		fmt.Printf("    idx: %d, msg: %s\n", idx, msg)
	//	}
}

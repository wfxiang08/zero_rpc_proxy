// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"strings"

	"github.com/c4pt0r/cfg"
	"github.com/wfxiang08/rpc_proxy/utils/log"
)

type Config struct {
	ProductName string
	ZkAddr      string

	ZkSessionTimeout int
	Verbose          bool

	Service   string
	FrontHost string
	FrontPort string
	IpPrefix  string
	BackAddr  string

	ProxyAddr string
}

func LoadConf(configFile string) (*Config, error) {
	c := cfg.NewCfg(configFile)
	if err := c.Load(); err != nil {
		log.PanicErrorf(err, "load config '%s' failed", configFile)
	}

	conf := &Config{}

	// 读取product
	conf.ProductName, _ = c.ReadString("product", "test")
	if len(conf.ProductName) == 0 {
		log.Panicf("invalid config: product entry is missing in %s", configFile)
	}

	// 读取zk
	conf.ZkAddr, _ = c.ReadString("zk", "")
	if len(conf.ZkAddr) == 0 {
		log.Panicf("invalid config: need zk entry is missing in %s", configFile)
	}
	conf.ZkAddr = strings.TrimSpace(conf.ZkAddr)

	loadConfInt := func(entry string, defInt int) int {
		v, _ := c.ReadInt(entry, defInt)
		if v < 0 {
			log.Panicf("invalid config: read %s = %d", entry, v)
		}
		return v
	}

	conf.ZkSessionTimeout = loadConfInt("zk_session_timeout", 30)
	conf.Verbose = loadConfInt("verbose", 0) == 1

	conf.Service, _ = c.ReadString("service", "")
	conf.Service = strings.TrimSpace(conf.Service)

	conf.FrontHost, _ = c.ReadString("front_host", "")
	conf.FrontHost = strings.TrimSpace(conf.FrontHost)

	conf.FrontPort, _ = c.ReadString("front_port", "")
	conf.FrontPort = strings.TrimSpace(conf.FrontPort)

	conf.IpPrefix, _ = c.ReadString("ip_prefix", "")
	conf.IpPrefix = strings.TrimSpace(conf.IpPrefix)

	conf.BackAddr, _ = c.ReadString("back_address", "")
	conf.BackAddr = strings.TrimSpace(conf.BackAddr)

	conf.ProxyAddr, _ = c.ReadString("proxy_address", "")
	conf.ProxyAddr = strings.TrimSpace(conf.ProxyAddr)
	return conf, nil
}

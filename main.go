package main

import (
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	_ "github.com/robberphex/beego-nacos-example/routers"
	"go.uber.org/multierr"
	"net"
	"os"
	"time"
)

const AppName = "test_app"

var ignoreNets = []string{
	"30.39.179.16/30",
	"fe80::1/24",
}
var allowNets = []string{}

type callback struct {
	ips     []string
	nclient naming_client.INamingClient
}

func newCallback() callback {
	ip := getRegIp()
	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(""), //When namespace is public, fill in the blank string here.
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("/tmp/nacos/log"),
		constant.WithCacheDir("/tmp/nacos/cache"),
		constant.WithLogLevel("debug"),
	)
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(
			"mse-96efa264-p.nacos-ans.mse.aliyuncs.com",
			8848,
			constant.WithScheme("http"),
		),
	}

	// Create naming client for service discovery
	nClient, _ := clients.CreateNamingClient(map[string]interface{}{
		constant.KEY_SERVER_CONFIGS: serverConfigs,
		constant.KEY_CLIENT_CONFIG:  clientConfig,
	})
	return callback{
		ips:     ip,
		nclient: nClient,
	}
}

func (c callback) AfterStart(app *beego.HttpServer) {
	fmt.Printf("current PID: %d, ips: %+v\n", os.Getpid(), c.ips)

	var totalErr error
	for _, ip := range c.ips {
		regParam := vo.RegisterInstanceParam{
			Ip:          ip,
			Port:        uint64(app.Cfg.Listen.HTTPPort),
			Weight:      100,
			Enable:      true,
			Healthy:     true,
			ServiceName: AppName,
			Ephemeral:   true,
		}
		_, err := c.nclient.RegisterInstance(regParam)
		totalErr = multierr.Append(totalErr, err)
	}
	fmt.Printf("%#v\n", totalErr)
}

func (c callback) BeforeShutdown(app *beego.HttpServer) {
	var totalErr error
	for _, ip := range c.ips {
		updateParam := vo.UpdateInstanceParam{
			Ip:          ip,
			Port:        uint64(app.Cfg.Listen.HTTPPort),
			Weight:      0,
			Enable:      false,
			ServiceName: AppName,
		}
		_, err := c.nclient.UpdateInstance(updateParam)
		totalErr = multierr.Append(totalErr, err)
	}
	fmt.Printf("%#v\n", totalErr)

	totalErr = nil
	for _, ip := range c.ips {
		deregParam := vo.DeregisterInstanceParam{
			Ip:          ip,
			Port:        uint64(app.Cfg.Listen.HTTPPort),
			ServiceName: AppName,
		}
		_, err := c.nclient.DeregisterInstance(deregParam)
		totalErr = multierr.Append(totalErr, err)
	}
	fmt.Printf("%#v\n", totalErr)
	time.Sleep(10 * time.Second)
}

func main() {
	beego.RunWithOptions(nil, beego.WithLifeCycleCallback(newCallback()))
}

func getRegIp() []string {
	var ignoreSubnets []*net.IPNet
	for _, ignoreNet := range ignoreNets {
		_, ignoreSubnet, _ := net.ParseCIDR(ignoreNet)
		ignoreSubnets = append(ignoreSubnets, ignoreSubnet)
	}

	var allowSubnets []*net.IPNet
	for _, allowNet := range allowNets {
		_, allowSubnet, _ := net.ParseCIDR(allowNet)
		allowSubnets = append(allowSubnets, allowSubnet)
	}

	var res []string

	ifaces, _ := net.Interfaces()
	// handle err
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
	addrLoop:
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				for _, ignoreSubnet := range ignoreSubnets {
					if ignoreSubnet.Contains(ipNet.IP) {
						continue addrLoop
					}
				}
				if len(allowSubnets) == 0 {
					res = append(res, ipNet.IP.String())
				} else {
					for _, allowSubnet := range allowSubnets {
						if allowSubnet.Contains(ipNet.IP) {
							res = append(res, ipNet.IP.String())
							continue addrLoop
						}
					}
				}
			}
		}
	}
	return res
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	service_contract_v1 "github.com/opensergo/opensergo-go/proto/service_contract/v1"
	_ "github.com/robberphex/example-beego-opensergo/routers"
	"go.uber.org/multierr"
	google_grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"
)

const AppName = "example-beego-opensergo"

var ignoreNets = []string{
	"30.39.179.16/30",
	"fe80::1/24",
}
var allowNets = []string{}

type callback struct {
	beego.UnimplementedLifeCycleCallback
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
	serverAddr := os.Getenv("serverAddr")
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(
			serverAddr,
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

	processServiceContract(app, c.ips)

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

func processServiceContract(app *beego.HttpServer, ips []string) {
	processedName := make(map[string]bool)
	service := service_contract_v1.ServiceDescriptor{}
	for _, info := range app.Handlers.GetAllControllerInfo() {
		method := service_contract_v1.MethodDescriptor{}
		fmt.Printf("=\t%s\t%#v\n", info.GetPattern(), info.GetMethod())
		method.HttpPaths = []string{info.GetPattern()}
		if len(info.GetMethod()) != 0 {
			for httpMethod, _ := range info.GetMethod() {
				method.HttpMethods = append(method.HttpMethods, httpMethod)
			}
			method.Name = strings.Join(method.HttpMethods, ",")
		} else {
			method.Name = "ALL"
			method.HttpMethods = []string{
				"GET",
				"POST",
				"PUT",
				"DELETE",
				"PATCH",
				"OPTIONS",
				"HEAD",
				"TRACE",
				"CONNECT",
				"MKCOL",
				"COPY",
				"MOVE",
				"PROPFIND",
				"PROPPATCH",
				"LOCK",
				"UNLOCK",
			}
		}
		method.Name += " " + info.GetPattern()
		if _, ok := processedName[method.Name]; !ok {
			service.Methods = append(service.Methods, &method)
			processedName[method.Name] = true
		}
	}
	var addrs []*service_contract_v1.SocketAddress
	for _, ip := range ips {
		addrs = append(addrs, &service_contract_v1.SocketAddress{
			Address:   ip,
			PortValue: uint32(app.Cfg.Listen.HTTPPort),
		})
	}

	req := service_contract_v1.ReportMetadataRequest{
		AppName: AppName,
		ServiceMetadata: []*service_contract_v1.ServiceMetadata{
			{
				ListeningAddresses: addrs,
				Protocols:          []string{"http"},
				ServiceContract: &service_contract_v1.ServiceContract{
					Services: []*service_contract_v1.ServiceDescriptor{
						&service,
					},
				},
			},
		},
	}
	ose := getOpenSergoEndpoint()
	timeoutCtx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := google_grpc.DialContext(timeoutCtx, ose, google_grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	mClient := service_contract_v1.NewMetadataServiceClient(conn)
	reply, err := mClient.ReportMetadata(context.Background(), &req)
	fmt.Printf("xxxxxxxxxxx: reply: %v, err: %v\n", reply, err)
	_ = reply
}

type openSergoConfig struct {
	Endpoint string `json:"endpoint"`
}

func getOpenSergoEndpoint() string {
	var err error
	configStr := os.Getenv("OPENSERGO_BOOTSTRAP_CONFIG")
	configBytes := []byte(configStr)
	if configStr == "" {
		configPath := os.Getenv("OPENSERGO_BOOTSTRAP")
		configBytes, err = ioutil.ReadFile(configPath)
		if err != nil {
			fmt.Printf("err: %v\n", err)
		}
	}
	config := openSergoConfig{}
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	return config.Endpoint
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
	beego.BeeApp.LifeCycleCallbacks = append(beego.BeeApp.LifeCycleCallbacks, newCallback())
	beego.Run()
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

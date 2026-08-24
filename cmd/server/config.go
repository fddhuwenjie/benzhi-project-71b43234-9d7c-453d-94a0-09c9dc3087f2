package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	address  string
	dataDir  string
	selftest bool
}

func parseConfig(arguments []string) (config, error) {
	set := flag.NewFlagSet("server", flag.ContinueOnError)
	address := set.String("addr", "", "HTTP 监听地址")
	dataDir := set.String("data", "./data", "本地数据目录")
	selftest := set.Bool("selftest", false, "运行完整流程自检后退出")
	if err := set.Parse(arguments); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("存在无法识别的参数")
	}
	resolved := strings.TrimSpace(*address)
	if resolved == "" {
		if portValue := strings.TrimSpace(os.Getenv("PORT")); portValue != "" {
			port, err := strconv.Atoi(portValue)
			if err != nil || port < 1 || port > 65535 {
				return config{}, fmt.Errorf("PORT 必须是 1 到 65535 之间的端口号")
			}
			resolved = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		} else {
			resolved = defaultAddress
		}
	}
	if err := validateAddress(resolved); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*dataDir) == "" {
		return config{}, fmt.Errorf("数据目录不能为空")
	}
	return config{address: resolved, dataDir: *dataDir, selftest: *selftest}, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("监听地址必须采用 host:port 格式: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("监听端口必须是 1 到 65535 之间的整数")
	}
	ip := net.ParseIP(host)
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("监听地址必须使用回环主机，拒绝通配或外部地址 %q", host)
	}
	return nil
}

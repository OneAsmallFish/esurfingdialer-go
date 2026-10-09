//go:build !linux

package netclient

import (
	"net"
	"runtime"

	"esurfingdialer/internal/logger"
)

// 非 Linux 平台忽略接口/mark 绑定
func setSocketOptions(_ *net.Dialer, iface string, fwmark uint32) {
	if iface != "" || fwmark != 0 {
		logger.Warnf("network interface/fwmark binding is unsupported on %s; parameters ignored", runtime.GOOS)
	}
}

// 非 Linux 平台忽略源地址绑定
func bindToSourceIP(_ *net.Dialer, ip string) {
	if ip != "" {
		logger.Warnf("source IP binding is unsupported on %s; parameter %q ignored", runtime.GOOS, ip)
	}
}

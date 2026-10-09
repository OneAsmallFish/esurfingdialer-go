//go:build linux

package netclient

import (
	"net"
	"syscall"

	"esurfingdialer/internal/logger"
)

// 绑定接口与 fwmark（本地策略路由与 mwan3 配合）
func setSocketOptions(dialer *net.Dialer, iface string, fwmark uint32) {
	if iface == "" && fwmark == 0 {
		return
	}
	dialer.Control = func(_, _ string, c syscall.RawConn) error {
		var controlErr error
		if err := c.Control(func(fd uintptr) {
			if iface != "" {
				if err := syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface); err != nil {
					controlErr = err
					return
				}
			}
			if fwmark != 0 {
				if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, int(fwmark)); err != nil {
					controlErr = err
					return
				}
			}
		}); err != nil {
			return err
		}
		return controlErr
	}
}

// 绑定源地址以触发对应策略路由
func bindToSourceIP(dialer *net.Dialer, ip string) {
	if ip == "" {
		return
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		logger.Warnf("invalid source IP %q, source binding disabled", ip)
		return
	}
	dialer.LocalAddr = &net.TCPAddr{IP: parsed}
}

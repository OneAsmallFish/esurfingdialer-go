package config

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
)

type Config struct {
	User     string `json:"user"`
	Password string `json:"password"`
	SMSCode  string `json:"sms_code"`
	LogLevel string `json:"log_level"`
	// 绑定出口接口（OpenWrt/mwan3 双 WAN 场景使用）
	BindInterface string `json:"bind_interface"`
	// 绑定出口源地址（部分设备需配合策略路由）
	BindSourceIP string `json:"bind_source_ip"`
	// 绑定 fwmark（与 mwan3 策略路由配合）
	BindFWMark string `json:"bind_fwmark"`
	// 独立校园 Portal。PortalAutoLogin 显式开启后才会提交账号密码。
	PortalURL       string `json:"portal_url"`
	PortalAutoLogin bool   `json:"portal_auto_login"`
	PortalUser      string `json:"portal_user"`
	PortalPassword  string `json:"portal_password"`
	PortalMAC       string `json:"portal_mac"`
	// 可选的旧客户端 UA 回退。返校日志显示 2089 与 2093 的服务端响应可能不同。
	UserAgent       string `json:"user_agent"`
	LegacyUserAgent string `json:"legacy_user_agent"`
}

func ParseArgs(args []string) (Config, error) {
	fs := flag.NewFlagSet("esurfingdialer", flag.ContinueOnError)

	var (
		cfg        Config
		configPath string
		userFlag   string
		passFlag   string
		smsFlag    string
		logFlag    string
		bindIface  string
		bindIP     string
		bindMark   string
		portalURL  string
		portalAuto bool
		portalUser string
		portalPass string
		portalMAC  string
	)

	fs.StringVar(&configPath, "c", "", "Config file path (json)")
	fs.StringVar(&configPath, "config", "", "Config file path (json)")
	fs.StringVar(&userFlag, "u", "", "Login user (phone number or other)")
	fs.StringVar(&userFlag, "user", "", "Login user (phone number or other)")
	fs.StringVar(&passFlag, "p", "", "Login password")
	fs.StringVar(&passFlag, "password", "", "Login password")
	fs.StringVar(&smsFlag, "s", "", "SMS verification code")
	fs.StringVar(&smsFlag, "sms", "", "SMS verification code")
	fs.StringVar(&logFlag, "log-level", "", "Log level (debug/info/warn/error)")
	fs.StringVar(&bindIface, "bind-interface", "", "Bind outgoing interface (OpenWrt/mwan3)")
	fs.StringVar(&bindIP, "bind-source-ip", "", "Bind source IP address (OpenWrt/mwan3)")
	fs.StringVar(&bindMark, "bind-fwmark", "", "Bind fwmark (OpenWrt/mwan3)")
	fs.StringVar(&portalURL, "portal-url", "", "Standalone campus portal URL")
	fs.BoolVar(&portalAuto, "portal-auto-login", false, "Automatically log in to the standalone portal")
	fs.StringVar(&portalUser, "portal-user", "", "Standalone portal login user (defaults to -u)")
	fs.StringVar(&portalPass, "portal-password", "", "Standalone portal login password (defaults to -p)")
	fs.StringVar(&portalMAC, "portal-mac", "", "MAC value submitted to the standalone portal")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if configPath != "" {
		fileCfg, err := LoadConfig(configPath)
		if err != nil {
			return Config{}, err
		}
		cfg = fileCfg
	}

	applyEnv(&cfg)
	applyFlags(&cfg, userFlag, passFlag, smsFlag, logFlag, bindIface, bindIP, bindMark, portalURL, portalAuto, portalUser, portalPass, portalMAC)

	if cfg.User == "" || cfg.Password == "" {
		return Config{}, errors.New("missing required -u/--user or -p/--password")
	}
	if cfg.PortalUser == "" {
		cfg.PortalUser = cfg.User
	}
	if cfg.PortalPassword == "" {
		cfg.PortalPassword = cfg.Password
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "CCTP/android64_vpn/2093"
	}
	if cfg.LegacyUserAgent == "" {
		cfg.LegacyUserAgent = "CCTP/android64_vpn/2089"
	}

	return cfg, nil
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("ESURFING_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("ESURFING_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("ESURFING_SMS"); v != "" {
		cfg.SMSCode = v
	}
	if v := os.Getenv("ESURFING_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("ESURFING_BIND_INTERFACE"); v != "" {
		cfg.BindInterface = v
	}
	if v := os.Getenv("ESURFING_BIND_SOURCE_IP"); v != "" {
		cfg.BindSourceIP = v
	}
	if v := os.Getenv("ESURFING_BIND_FWMARK"); v != "" {
		cfg.BindFWMark = v
	}
	if v := os.Getenv("ESURFING_PORTAL_URL"); v != "" {
		cfg.PortalURL = v
	}
	if v := os.Getenv("ESURFING_PORTAL_AUTO_LOGIN"); v == "1" || v == "true" {
		cfg.PortalAutoLogin = true
	}
	if v := os.Getenv("ESURFING_PORTAL_USER"); v != "" {
		cfg.PortalUser = v
	}
	if v := os.Getenv("ESURFING_PORTAL_PASSWORD"); v != "" {
		cfg.PortalPassword = v
	}
	if v := os.Getenv("ESURFING_PORTAL_MAC"); v != "" {
		cfg.PortalMAC = v
	}
	if v := os.Getenv("ESURFING_USER_AGENT"); v != "" {
		cfg.UserAgent = v
	}
	if v := os.Getenv("ESURFING_LEGACY_USER_AGENT"); v != "" {
		cfg.LegacyUserAgent = v
	}
}

func applyFlags(cfg *Config, user, pass, sms, logLevel, bindIface, bindIP, bindMark, portalURL string, portalAuto bool, portalUser, portalPass, portalMAC string) {
	if user != "" {
		cfg.User = user
	}
	if pass != "" {
		cfg.Password = pass
	}
	if sms != "" {
		cfg.SMSCode = sms
	}
	if logLevel != "" {
		cfg.LogLevel = logLevel
	}
	if bindIface != "" {
		cfg.BindInterface = bindIface
	}
	if bindIP != "" {
		cfg.BindSourceIP = bindIP
	}
	if bindMark != "" {
		cfg.BindFWMark = bindMark
	}
	if portalURL != "" {
		cfg.PortalURL = portalURL
	}
	if portalAuto {
		cfg.PortalAutoLogin = true
	}
	if portalUser != "" {
		cfg.PortalUser = portalUser
	}
	if portalPass != "" {
		cfg.PortalPassword = portalPass
	}
	if portalMAC != "" {
		cfg.PortalMAC = portalMAC
	}
}

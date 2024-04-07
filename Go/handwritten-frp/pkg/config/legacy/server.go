package legacy

import "handwritten-projects/go/handwritten-frp/pkg/auth/legacy"

type HTTPPluginOptions struct {
	Name      string   `ini:"name"`
	Addr      string   `ini:"addr"`
	Path      string   `ini:"path"`
	Ops       []string `ini:"ops"`
	TLSVerify bool     `ini:"tlsVerify"`
}

// ServerCommonConf 包含了一个服务端服务的信息。
// 推荐使用GetDefaultServerConf而不是直接创建这个对象，
// 这样所有的未指定字段都有一个合理的默认值。
type ServerCommonConf struct {
	legacy.ServerConfig `ini:",extends"`

	// BindAddr 指定了服务器绑定的地址，默认值为"0.0.0.0"
	BindAddr string `ini:"bind_addr" json:"bind_addr"`
	// BindPort 制定了服务器监听的端口，默认值为7000
	BindPort int `ini:"bind_port" json:"bind_port"`
	// KCPBindPort 指定了服务器监听的KCP端口，如果这个值等于0
	// 那么服务不会监听KCP连接，默认值为0。
	// KCP是一种基于udp的可靠传输协议，旨在提供更可靠和更高效的数据传输。
	// KCP连接通常用于网络游戏、视频流媒体、实时通信等对传输延迟和稳定性要求较高的
	// 的场景，因为它能够在高丢包率和不稳定的网络环境下保证较低的延迟和更可靠的数据传输。
	KCPBindPort int `ini:"kcp_bind_port" json:"kcp_bind_port"`
	// QUICBindPort 指定了服务器监听的QUIC端口
	// 设置0将会禁用这个特征，默认值等于0。
	QUICBindPort int `ini:"quic_bind_port" json:"quic_bind_port"`
	// QUICKeepalivePeriod 指定了连接的保活周期，即在没有数据传输时，连接需要发送保活
	// 数据包以确保连接保持活动状态。这个参数可以防止NAT等设备在长时间没有活动的连接上执行
	// 超时处理，以保证连接的存活性。而这个参数就指定了保活数据包发送的时间间隔。较短的
	// 保活周期会增加带宽消耗，但是可以更快的检测到连接的故障。
	QUICKeepalivePeriod int `ini:"quic_keepalive_period" json:"quic_keepalive_period"`
	// QUICMaxIdleTimeout 指定了连接的最大空闲超时时间。当连接在该时间段内没有数据传输时
	// 连接将被视为空闲连接从而被关闭。这个参数用于确保不会再网络中保持空闲连接，并减少网络负载。
	// 默认情况下这个值等于0，表示禁用空闲超时，即不会自动关闭空闲连接。
	QUICMaxIdleTimeout int `ini:"quic_max_idle_timeout" json:"quic_max_idle_timeout"`
	// QUICMaxIncommingStreams 指定了连接允许的最大同时接受的流的数量。QUIC支持多路复用，可以在
	// 单个连接上同时传输多个数据流。该参数限制了连接上可以同时接受的流的数量，可以用于控制连接
	// 的负载和资源消耗。默认情况下，该参数的值为0，表示没有限制。
	QUICMaxIncommingStreams int `ini:"quic_max_incoming_streams" json:"quic_max_incoming_streams"`
	// ProxyBindAddr 指定了proxy绑定的地址，这个参数可能和BindAddr一致
	ProxyBindAddr string `ini:"proxy_bind_addr" json:"proxy_bind_addr"`
	// VhostHTTPPort 指定了服务器监听HTTP Vhost请求的端口，如果值为0，则不会监听
	// HTTP请求。
	// 所谓HTTP Vhost请求是指通过HTTP协议发送到虚拟主机的请求。在Web服务器配置中
	// 可以配置多个虚拟主机，每个虚拟主机都拥有自己的域名或IP地址，并提供不同的网站内容或
	// 服务。http请求中的`host`字段指定了应该路由到哪个虚拟主机。
	// 通过使用HTTP Vhost请求, 可以在单个服务器上托管多个不同域名的网站。
	// 相比于普通HTTP请求，HTTP Vhost请求使用了host头。
	VhostHTTPPort int `ini:"vhost_http_port" json:"vhost_http_port"`
	// VhostHTTPSPort 指定了服务器监听HTTPS Vhost 请求，如果这个值为0，那么这个服务器
	// HTTPS请求。
	VhostHTTPSPort int `ini:"vhost_https_port" json:"vhost_https_port"`
	// TCPMuxHTTPConnectPort 指定了服务器监听的TCP HTTP连接请求，如果值为0
	// 那么服务器就不会支持单个端口的多路复用TCP请求，如果不为0，那么它会监听HTTP连接请求。
	TCPMuxHTTPConnectPort int `ini:"tcpmux_httpconnect_port" json:"tcpmux_httpconnect_port"`
	// TCPMuxPassthrough 如果为true，那么frps不会对流量进行任何更新
	TCPMuxPassthrough bool `ini:"tcpmux_passthrough" json:"tcpmux_passthrough"`
	// VhostHTTPTimeout 指定了Vhost HTTP 服务器http响应头的超时时间，单位为秒，默认值为60
	VhostHTTPTimeout int64 `ini:"vhost_http_timeout" json:"vhost_http_timeout"`
	// DashboardAddr 指定了Dashboard 绑定的地址，默认值为"0.0.0.0"
	DashboardAddr string `ini:"dashboard_addr" json:"dashboard_addr"`
	// DashboardPort 指定了dashboard port监听的地址，如果这个值为0，这个dashboard将不会启动
	// 默认值为0。
	DashboardPort int `ini:"dashboard_port" json:"dashboard_port"`
	// DashboardTLSCertFile 指定了服务器加载证书文件的路径，如果dashboard_tls_cert_file、
	// dashboard_tls_key_file是有效的，服务器将可以支持tls配置。
	DashboardTLSCertFile string `ini:"dashboard_tls_cert_file" json:"dashboard_tls_cert_file"`
	// DashboardTLSKeyFile 指定了服务器需要加载的密钥路径。
	DashboardTLSKeyFile string `ini:"dashboard_tls_key_file" json:"dashboard_tls_mode"`
	// DashboardUser 指定了登录dashboard的用户的名称。
	DashboardUser string `ini:"dashboard_user" json:"dashboard_user"`
	// DashboardPwd 指定了用户登录的密码。
	DashboardPwd string `ini:"dashboard_pwd" json:"dashboard_pwd"`
	// EnablePrometheus 将在{dashboard_addr}:{dashboard_port}上的metrics API导出Prometheus指标
	// Prometheus指标是用于监控和度量应用程序性能、状态和其他相关信息的数据点。
	EnablePrometheus bool `ini:"enable_prometheus" json:"enable_prometheus"`
	// AssetsDir 指定了dashboard加载资源的本地目录路径，如果这个值为空字符串，
	// 则将从捆绑的可执行文件中使用statik加载资源。默认情况下，该值为""。
	AssetsDir string `ini:"assets_dir" json:"assets_dir"`
	// LogFile 指定了日志将会写入的文件名称，只有当LogWay被正确设置之后，才会被使用
	// 默认值为"console"。
	LogFile string `ini:"log_file" json:"log_file"`
	// LogWay 指定了日志管理方式，有效值为"console"或"file"。如果使用"console"，日志将
	// 打印到标准输出(stdout)中。如果使用"file"，日志将打印到LogFile中。默认情况下，该
	// 值为"console"
	LogWay string `ini:"log_way" json:"log_way"`
	// LogLevel 指定了log level的最小值，可用的值包括"trace"、"debug"、"info"、"warn"
	// and "error"， 默认为"info"
	LogLevel string `ini:"log_level" json:"log_level"`
	//LogMaxDays 指定了在删除日志之前，存储日志信息的最大天数, 只有当LogWay为"file"时，
	// 这个值才有用，默认这个值为0。
	LogMaxDays int64 `ini:"log_max_days" json:"log_max_days"`
	// DisableLogColor 当LogWay == "console"时，设置这个值为"true"，就可以关闭彩色的log日志。
	// 默认这个值为false。
	DisableLogColor bool `ini:"double_log_color" json:"disable_log_color"`
	// DetailedErrorsToClient 定义了传输带有debug信息的特定error给frpc，默认情况下，这个
	// 值为true。
	DetailedErrorsToClient bool `ini:"detailed_errors_to_client" json:"detailed_errors_to_client"`
	// SubDomainHost 指定了附加到子域的域，当客户端使用Vhost代理请求时，比如这个值如果
	// 设置为"frps.com"，并且客户端请求子域"test"，那么URL将是"test.frps.com"，默认这个值
	// 为""。
	SubDomainHost string `ini:"subdomain_host" json:"subdomain_host"`
	// TCPMux 可以触发TCP流复用，这允许多个请求从客户端共享单个TCP连接，默认情况下该值为true。
	TCPMux bool `ini:"tcp_mux" json:"tcp_mux"`
	// TCPMuxKeepaliveInterval 指定了TCP流付哦路复用器的保活间隔
	// 如果TCPMux为true，则应用层的心跳是不必要的，因为它可以值依赖TCPMux中的心跳。
	TCPMuxKeepaliveInterval int64 `ini:"tcp_mux_keepalive_interval" json:"tcp_mux_keepalive_interval"`
	// TCPKeepAlive 指定了frpc和frps之间活动网络连接的保活探测间隔，如果为负数，则禁用保活探测。
	TCPKeepAlive int64 `ini:"tcp_keepalive" json:"tcp_keepalibe"`
	// Custom404Page 指定了自定义404页面的路径。如果该值为""，则会显示默认页面。
	// 默认情况下，该值为""。
	Custom404Page string `ini:"custom_404_page" json:"custom_404_page"`
	// AllowPorts 指定了一连串客户端可以代理到的一组端口。如果该值的长度为0，
	// 则允许所有端口。默认情况下，该值是一个空集合。
	AllowPorts map[string]struct{} `ini:"-" json:"-"`
	// 上面字段类型的字符串形式。
	AllowPortsStr string `ini:"-" json:"-"`
	// MaxPoolCount 为每个代理设置了最大的连接池大小，该值默认为5。
	MaxPoolCount int64 `ini:"max_pool_count" json:"max_pool_count"`
	// MaxPortsPerClient 指定了单个客户端可以代理到的最大端口数。如果该值为0，则
	// 不会应用任何限制。默认情况下，该值为0。
	MaxPortsPerClient int64 `ini:"max_ports_per_client" json:""max_ports_per_client`
	// TLSOnly 指定了是否仅接受TLS加密连接。默认情况下，该值为false。
	TLSOnly bool `ini:"tls-only" json:"tls_only"`
	// TLSCertFile 指定了服务器加载的证书文件的路径。如果
	// "tls_cert_file"、"tls_key_file"是有效的,服务器将使用提供的TLS配置。
	// 否则，服务器将使用自动生成的TLS配置。
	TLSCertFile string `ini:"tls_cert_file" json:"tls_cert_file"`
	// TLSKeyFile 指定服务器将加载密钥文件的路径。如果
	// "tls_cert_file"、"tls_key_file"是有效的,服务器将使用提供的TLS配置。
	// 否则，服务器将使用自动生成的TLS配置。
	TLSKeyFile string `ini:"tls_key_file" json:"tls_key_file"`
	// TLSTrustedCaFile 指定了服务器将加载的客户端证书的路径。仅当"tls_only"
	// true时才有效。如果"tls_trusted_ca_file"有效，则服务器将验证每个客户端的
	// 证书
	TLSTrustedCaFile string `ini:"tls_trusted_ca_file" json:"tls_trusted_ca_file"`
	// HeartbeatTimeout 指定了在终止连接之前等待心跳的最大时间。不建议更改
	// 此值，默认情况下，该值为90.设置负值可以禁用它。
	HeartbeatTimeout int64 `ini:"heartbeat_timeout" json:"heartbeat_timeout"`
	// UserConnTimeout 指定了等待工作连接的最大时间，默认情况下，该值为0。
	UserConnTimeout int64 `ini:"user_conn_timeout" json:"user_conn_timeout"`
	// HTTPPlugins 指定了支持HTTP协议的服务端插件。
	HTTPPlugins map[string]HTTPPluginOptions `ini:"-" json:"http_plugins"`
	// UDPPacketSize 指定了UDP包大小，默认情况下，该值为1500。
	UDPPacketSize int64 `ini:"udp_packet_size" json:"udp_packet_size"`
	// PprofEnable 指定了在dashboard监听器中是否使用pprof
	// 要使用的话，必须先设置Dashboard 端口
	PprofEnable bool `ini:"pprof_enable" json:"pprof_enable"`
	// NatHoleAnalysisDataReserveHours 指定了NAT穿透分析数据的小时数。
	NatHoleAnalysisDataReserveHours int64 `ini:"nat_hole_analysis_data_reverve_hours" json:"nat_hole_anaylysis_data_reverve_hours"`
}

func UnmarshalServerConfFromIni(source interface{})

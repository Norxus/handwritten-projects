package legacy

type BaseConfig struct {
	// AuthenticationMethod 指定了frpc和frps之间身份的身份验证方法
	// 如果token已经被指定了，令牌将被读取到登录消息中。如果执行了"oidc",将使用OIDC(Open ID Connect)
	// 设置发出OIDC令牌。默认情况下，该值为"token"
	AuthenticationMethod string `ini:"authentication_method" json:"authentication_method"`
	// AuthenticateHeartBeats 指定了是否需要包含鉴权token在心跳中发给frps，默认为false
	AuthenticateHeartBeats bool `ini:"authenticate_heartbeats" json:"authenticate_heartbeats"`
	// AuthenticateNewWorkConns 指定了是否需要包括鉴权token在新的网络连接中发送给frps
	// 默认情况下，这个值为false
	AuthenticateNewWorkConns bool `ini:"authenticate_new_work_conns" json:"authenticate_new_work_conns"`
}

// 为BaseConfig设置默认值
func getDefaultBaseConf() BaseConfig {
	return BaseConfig{
		AuthenticationMethod:     "token",
		AuthenticateHeartBeats:   false,
		AuthenticateNewWorkConns: false,
	}
}

type OidcClientConfig struct {
	// OidcClienrID 指定的client id用于在OIDC权限认证中获取一个token
	// 也就是当AuthenticationMethod == "oidc"。默认情况下，这个值为""。
	OidcClientID string `ini:"oidc_client_id" json:"oidc_client_id"`
	// OidcClientSecret 指定了客户端密钥用于在OIDC认证中获取一个token
	// 也就是当AuthenticationMethod = "oidc"。默认情况下，这个值为""。
	OidcClientSecret string `ini:"oidc_client_secret" json:"oidc_client_secret"`
	// OidcAudience 指定了OIDC身份验证中token的受众
	// 也就是当AuthenticationMethod = "oidc"。默认情况下，这个值为""。
	OidcAudience string `ini:"oidc_audience" json:"oidc_audience"`
	// OidcScope 指定了OIDC身份验证中token的范围
	// 也就是当 AuthenticationMethod = "oidc"。默认情况下，这个值为""。
	OidcScope string `ini:"oidc_scope" json:"oidc_scope"`
	// OidcTokenEndPointURL 指定了实现OIDC Token EndPoint的URL
	// 也就是当 AuthenticationMethod = "oidc"。 默认情况下，这个值为""。
	OidcTokenEndPointURL string `ini:"oidc_token_endpoint_url" json:"oidc_token_endpoint_url"`
	// OidcAdditionalEndpointParams 指定了用于发送的额外参数，
	// 这个字段将会被传输到OIDC令牌生成器的map[string][]string中，
	// 该字段将会设置前缀"oidc_additional_"
	OidcAdditionalEndpointParams map[string]string `ini:"-" json:"oidc_additional_endpoint_params"`
}

func getDefaultOidcClientConf() OidcClientConfig {
	return OidcClientConfig{
		OidcClientID:                 "",
		OidcClientSecret:             "",
		OidcAudience:                 "",
		OidcScope:                    "",
		OidcTokenEndPointURL:         "",
		OidcAdditionalEndpointParams: make(map[string]string),
	}
}

type OidcServerConfig struct {
	// OidcIssuer 指定用于验证OIDC令牌的发行者。该发行者将用于加载公钥以验证签名。
	// 并将与OIDC令牌中的发行者声明进行比较。如果AuthenticationMethod == "oidc"，则将
	// 使用它。默认情况下该值为""。
	OidcIssuer string `ini:"oidc_issuer" json:"oidc_issuer"`
	// OidcAudience 指定在验证时OIDC令牌应该包含的受众，如果该值为空，
	// 则会跳过受众（"客户端ID"）验证。当AuthenticationMethod == "oidc"时，
	// 将使用该值，默认情况下，该值为空字符串。
	OidcAudience string `ini:"oidc_audience" json:"oidc_audience"`
	// OidcSkipExpiryCheck 指定了如果OIDC的token过期了之后是否跳过检查。
	// 当AuthenticationMethod == "oidc"时，将使用该值，默认情况下，该值为空字符串。
	OidcSkipExpiryCheck bool `ini:"oidc_skip_expiry_check" json:"oidc_skip_expiry_check"`
	// oidcSkipIssuerCheck 指定了当OIDC的token的发行者声明是否与OidcIssuer中指定的发行者匹配
	// 当AuthenticationMethod = "oidc"式，将使用该值，默认情况下，这个值为false
	oidcSkipIssuerCheck bool `ini:"oidc_skip_issuer_check" json:"oidc_skip_issuer_check"`
}

func getDefaultOidcServerConf() OidcServerConfig {
	return OidcServerConfig{
		OidcIssuer:          "",
		OidcAudience:        "",
		OidcSkipExpiryCheck: false,
		oidcSkipIssuerCheck: false,
	}
}

type TokenConfig struct {
	// Token 指定了用于创建要发送到服务器的密钥的token，
	// 服务器必须具有匹配的token才能成功进行授权。默认情况下，这个值为空字符串。
	Token string `ini:"token" json:"token"`
}

func getDefaultTokenConf() TokenConfig {
	return TokenConfig{
		Token: "",
	}
}

type ClientConfig struct {
	BaseConfig       `ini:",extends"`
	OidcClientConfig `ini:",extends"`
	TokenConfig      `ini:",extends"`
}

type ServerConfig struct {
	BaseConfig       `ini:",extends"`
	OidcServerConfig `ini:",extends"`
	TokenConfig      `ini:",extends"`
}

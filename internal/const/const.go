package teaconst

const (
	Version = "0.2.0"

	ProductName   = "Edge API"
	ProcessName   = "edge-api"
	ProductNameZH = "Edge"

	Role = "api"

	EncryptKey    = "8f983f4d69b83aaa0d74b21a212f6967"
	EncryptMethod = "aes-256-cfb"

	ErrServer = "服务器出了点小问题，请稍后重试"

	SystemdServiceName = "edge-api"

	// 其他节点版本号，用来检测是否有需要升级的节点

	NodeVersion          = "0.2.0"
	UserNodeVersion      = "0.0.7"
	AuthorityNodeVersion = "0.0.1"
	MonitorNodeVersion   = "0.0.1"
	DNSNodeVersion       = "0.0.1"
)

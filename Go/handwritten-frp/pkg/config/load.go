package config

import (
	"handwritten-projects/go/handwritten-frp/pkg/config/legacy"
	"os"

	"google.golang.org/genproto/googleapis/appengine/legacy"
	"gopkg.in/ini.v1"

	v1 "github.com/fatedier/frp/pkg/config/v1"
)

func LoadServerConfig(path string, strict bool) (*v1.ServerConfig, bool, error) {
	var (
		svrCfg         *v1.ServerConfig
		isLegacyFormat bool
	)
	// detect legacy ini format
	if DetectLegacyINIFormatFromFile(path) {
		content, err := legacy.GetRenderedConfFromFile(path)
		if err != nil {
			return nil, true, err
		}
		legacyCfg, err := legacy.Um
	}
}

// 利用os包读取文件形成字节流
func DetectLegacyINIFormatFromFile(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return DetectLegacyINIFormat(b)
}

// 加载解析INI格式的配置文件内容，查看是不是有common配置
// ini格式阁下：
/*
   [database]
   host = localhost
   port = 3306
   user = root
   password = password123
*/
func DetectLegacyINIFormat(content []byte) bool {
	f, err := ini.Load(content)
	if err != nil {
		return false
	}
	if _, err := f.GetSection("common"); err == nil {
		return true
	}
	return false
}

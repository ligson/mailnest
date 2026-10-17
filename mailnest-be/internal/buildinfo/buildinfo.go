// Package buildinfo 提供随二进制固化的版本信息，不读取生产配置或访问数据库。
package buildinfo

import (
	_ "embed"
	"strings"
)

// 默认版本随源码发布；正式构建可通过 -ldflags -X 覆盖这些字段。
//
//go:embed version.txt
var sourceVersion string

var (
	Version   string
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

func Current() Info {
	version := strings.TrimSpace(Version)
	if version == "" {
		version = strings.TrimSpace(sourceVersion)
	}
	return Info{Version: version, Commit: valueOrUnknown(Commit), BuildTime: valueOrUnknown(BuildTime)}
}

func valueOrUnknown(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "unknown"
}

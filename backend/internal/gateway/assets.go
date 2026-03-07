package gateway

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed webdist/*
var webDistFS embed.FS

// GetWebAssets 实现 sdk.WebAssetsProvider 接口
// 返回嵌入的前端静态资源，供核心提取后通过 HTTP 提供服务
func (g *OpenAIGateway) GetWebAssets() map[string][]byte {
	assets := make(map[string][]byte)
	fs.WalkDir(webDistFS, "webdist", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		content, err := webDistFS.ReadFile(path)
		if err != nil {
			return nil
		}
		// 去掉 "webdist/" 前缀，保留相对路径
		relPath := strings.TrimPrefix(path, "webdist/")
		assets[relPath] = content
		return nil
	})
	return assets
}

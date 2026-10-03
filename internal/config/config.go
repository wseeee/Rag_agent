package config

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

const (
	MilvusDBName         = "agent"
	MilvusCollectionName = "biz"
)

var FileDir = "./docs/"

func init() {
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		_ = adapter.AddPath("etc/config", "etc")
	}
}

package config

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

const (
	MilvusDBName         = "agent"
	MilvusCollectionName = "biz"

	DefaultMilvusAddr      = "localhost:19530"
	DefaultMilvusDefaultDB = "default"

	DefaultRedisAddr      = "127.0.0.1:6379"
	DefaultRedisPassword  = ""
	DefaultRedisDB        = 1
	DefaultMinIOEndpoint  = "127.0.0.1:9000"
	DefaultMinIOAccessKey = "minioadmin"
	DefaultMinIOSecretKey = "minioadmin"
	DefaultMinIOBucket    = "superbiz-documents"
	DefaultKafkaBroker    = "127.0.0.1:9092"
	DefaultKafkaTopic     = "superbiz-file-processing"
	DefaultKafkaGroupID   = "superbiz-file-indexer-group"
)

var FileDir = "./docs/"

func init() {
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		_ = adapter.AddPath("etc/config", "etc")
	}
}


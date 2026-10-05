package config

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/util/gconv"
)

var (
	FileDir = "./docs/"
	// C 全局强类型配置单例
	C        Config
	initOnce sync.Once
)

type Config struct {
	Server    ServerConfig    `json:"server" yaml:"server"`
	Logger    LoggerConfig    `json:"logger" yaml:"logger"`
	FileDir   string          `json:"file_dir" yaml:"file_dir"`
	McpUrl    string          `json:"mcp_url" yaml:"mcp_url"`
	Redis     RedisConfig     `json:"redis" yaml:"redis"`
	MinIO     MinIOConfig     `json:"minio" yaml:"minio"`
	Kafka     KafkaConfig     `json:"kafka" yaml:"kafka"`
	Tika      TikaConfig      `json:"tika" yaml:"tika"`
	Milvus    MilvusConfig    `json:"milvus" yaml:"milvus"`
	Retriever RetrieverConfig `json:"retriever" yaml:"retriever"`
	MySQL     MySQLConfig     `json:"mysql" yaml:"mysql"`
	Memory    MemoryConfig    `json:"memory" yaml:"memory"`
	Sentry    SentryConfig    `json:"sentry" yaml:"sentry"`

	DsThinkChatModel     ModelConfig `json:"ds_think_chat_model" yaml:"ds_think_chat_model"`
	DsQuickChatModel     ModelConfig `json:"ds_quick_chat_model" yaml:"ds_quick_chat_model"`
	DoubaoEmbeddingModel ModelConfig `json:"doubao_embedding_model" yaml:"doubao_embedding_model"`
}

type ServerConfig struct {
	Address     string `json:"address" yaml:"address"`
	OpenapiPath string `json:"openapiPath" yaml:"openapiPath"`
	SwaggerPath string `json:"swaggerPath" yaml:"swaggerPath"`
}

type LoggerConfig struct {
	Level  string `json:"level" yaml:"level"`
	Stdout bool   `json:"stdout" yaml:"stdout"`
}

type ModelConfig struct {
	ApiKey  string `json:"api_key" yaml:"api_key"`
	BaseUrl string `json:"base_url" yaml:"base_url"`
	Model   string `json:"model" yaml:"model"`
}

type RedisConfig struct {
	Addr     string `json:"addr" yaml:"addr"`
	Password string `json:"password" yaml:"password"`
	DB       int    `json:"db" yaml:"db"`
}

type MinIOConfig struct {
	Endpoint  string `json:"endpoint" yaml:"endpoint"`
	AccessKey string `json:"accessKey" yaml:"accessKey"`
	SecretKey string `json:"secretKey" yaml:"secretKey"`
	UseSSL    bool   `json:"useSSL" yaml:"useSSL"`
	Bucket    string `json:"bucket" yaml:"bucket"`
}

type KafkaConfig struct {
	Brokers []string `json:"brokers" yaml:"brokers"`
	Topic   string   `json:"topic" yaml:"topic"`
	GroupID string   `json:"group_id" yaml:"group_id"`
}

type TikaConfig struct {
	ServerUrl string `json:"server_url" yaml:"server_url"`
}

type MilvusConfig struct {
	Addr           string `json:"addr" yaml:"addr"`
	DefaultDB      string `json:"default_db" yaml:"default_db"`
	DBName         string `json:"db_name" yaml:"db_name"`
	CollectionName string `json:"collection_name" yaml:"collection_name"`
}

type RetrieverConfig struct {
	TopK        int     `json:"top_k" yaml:"top_k"`
	DenseWeight float64 `json:"dense_weight" yaml:"dense_weight"`
	Bm25Weight  float64 `json:"bm25_weight" yaml:"bm25_weight"`
}

type MySQLConfig struct {
	DSN          string `json:"dsn" yaml:"dsn"`
	MaxIdleConns int    `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxOpenConns int    `json:"max_open_conns" yaml:"max_open_conns"`
}

type MemoryConfig struct {
	ShortTermWindow     int     `json:"short_term_window" yaml:"short_term_window"`
	SummaryTriggerTurns int     `json:"summary_trigger_turns" yaml:"summary_trigger_turns"`
	TTLDays             int     `json:"ttl_days" yaml:"ttl_days"`
	LongTermThreshold   float32 `json:"long_term_threshold" yaml:"long_term_threshold"`
	TopK                int     `json:"top_k" yaml:"top_k"`
	MilvusCollection    string  `json:"milvus_collection" yaml:"milvus_collection"`
}

type SentryConfig struct {
	Dsn       string `json:"dsn" yaml:"dsn"`
	AuthToken string `json:"auth_token" yaml:"auth_token"`
	Org       string `json:"org" yaml:"org"`
	Project          string  `json:"project" yaml:"project"`
	BaseUrl          string  `json:"base_url" yaml:"base_url"`
	TracesSampleRate float64 `json:"traces_sample_rate" yaml:"traces_sample_rate"`
}

func ensureAdapter() {
	initOnce.Do(func() {
		if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
			_ = adapter.AddPath("etc/config", "etc")
		}
	})
}

// Init 解析配置文件并填充到强类型全局配置结构体 C 中
func Init(ctx context.Context) error {
	ensureAdapter()
	data, err := g.Cfg().Data(ctx)
	if err != nil {
		return err
	}
	if err := gconv.Scan(data, &C); err != nil {
		return err
	}
	if C.FileDir != "" {
		FileDir = C.FileDir
	}
	return nil
}

// MustLoad 初始化配置，失败则 panic
func MustLoad(ctx context.Context) {
	if err := Init(ctx); err != nil {
		panic(err)
	}
}

func init() {
	ensureAdapter()
	// 在包导入时进行尽力加载，如果配置存在则预填 C
	_ = Init(context.Background())
}

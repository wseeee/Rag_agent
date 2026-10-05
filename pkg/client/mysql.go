package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	cfg "SuperBizAgent/internal/config"

	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB      *gorm.DB
	mysqlMu sync.Mutex
)

// InitMySQL 初始化全局 GORM 数据库连接与连接池
func InitMySQL(dsn string) (*gorm.DB, error) {
	mysqlMu.Lock()
	defer mysqlMu.Unlock()

	if DB != nil {
		return DB, nil
	}

	if dsn == "" {
		dsn = cfg.C.MySQL.DSN
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to mysql via gorm failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	maxIdle := cfg.C.MySQL.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	maxOpen := cfg.C.MySQL.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 50
	}

	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	g.Log().Info(context.Background(), "[GORM] MySQL 数据库连接池初始化成功")
	return DB, nil
}

// GetDB 获取全局 GORM 实例，未显式初始化时自动读取配置加载
func GetDB() *gorm.DB {
	if DB == nil {
		_, _ = InitMySQL(cfg.C.MySQL.DSN)
	}
	return DB
}

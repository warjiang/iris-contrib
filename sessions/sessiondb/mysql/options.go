package mysql

import (
	"fmt"
	"gorm.io/gorm"
)

type Options struct {
	Host      string
	Port      int
	Username  string
	Password  string
	DataBase  string
	TableName string
	GormDB    *gorm.DB
}

func (opts *Options) GenerateDSN() string {
	database := opts.DataBase
	if len(database) == 0 {
		database = "iris"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		opts.Username, opts.Password, opts.Host, opts.Port, database)
	return dsn
}

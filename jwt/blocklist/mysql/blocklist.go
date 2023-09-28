package redis

import (
	"context"
	"fmt"
	"github.com/kataras/iris/v12/middleware/jwt"
	_ "github.com/kataras/iris/v12/sessions/sessiondb/redis"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var defaultContext = context.Background()

type MysqlOptions struct {
	Host     string
	Port     int
	User     string
	Password string
	DataBase string
	GormDB   *gorm.DB
}

// Blocklist is a jwt.Blocklist backed by Redis.
type Blocklist struct {
	MysqlOptions *MysqlOptions
}

var _ jwt.Blocklist = (*Blocklist)(nil)

// NewBlocklist returns a new redis-based Blocklist.
// Modify its ClientOptions or ClusterOptions depending the application needs
// and call its Connect.
//
// Usage:
//
//	blocklist := NewBlocklist()
//	blocklist.ClientOptions.Addr = ...
//	err := blocklist.Connect()
//
// And register it:
//
//	verifier := jwt.NewVerifier(...)
//	verifier.Blocklist = blocklist
func NewBlocklist() *Blocklist {
	return &Blocklist{}
}

// Connect prepares the redis client and fires a ping response to it.
func (b *Blocklist) Connect() error {
	opts := b.MysqlOptions
	if opts.GormDB != nil {
		return nil
	}
	database := opts.DataBase
	if len(database) == 0 {
		database = "iris"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", opts.User, opts.Password, opts.Host, opts.Port, database)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       dsn,
		DefaultStringSize:         256,
		DisableDatetimePrecision:  true,
		DontSupportRenameIndex:    true,
		DontSupportRenameColumn:   true,
		SkipInitializeWithVersion: false,
	}))
	if err != nil {
		return err
	}
	b.MysqlOptions.GormDB = gormDB
	return nil
}

// IsConnected reports whether the Connect function was called.
func (b *Blocklist) IsConnected() bool {
	sqlDB, err := b.MysqlOptions.GormDB.DB()
	if err != nil {
		return false
	}
	if err = sqlDB.Ping(); err != nil {
		return false
	}
	return true
}

// ValidateToken checks if the token exists and
func (b *Blocklist) ValidateToken(token []byte, c jwt.Claims, err error) error {
	return nil
}

// InvalidateToken invalidates a verified JWT token.
func (b *Blocklist) InvalidateToken(token []byte, c jwt.Claims) error {
	return nil
}

// Del removes a token from the storage.
func (b *Blocklist) Del(key string) error {
	return nil
}

// Has reports whether a specific token exists in the storage.
func (b *Blocklist) Has(key string) (bool, error) {
	return false, nil
}

// Count returns the total amount of tokens stored.
func (b *Blocklist) Count() (int64, error) {
	return 0, nil
}

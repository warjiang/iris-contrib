package mysql

import (
	"errors"
	"fmt"
	"github.com/kataras/golog"
	"github.com/kataras/iris/v12/sessions"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"time"
)

var _ sessions.Database = (*Database)(nil)

// New returns a new mysql sessions database.
func New(opts Options) *Database {
	db := opts.GormDB
	if db == nil {
		dsn := opts.GenerateDSN()
		gormDB, err := gorm.Open(mysql.New(mysql.Config{
			DSN:                       dsn,
			DefaultStringSize:         256,
			DisableDatetimePrecision:  true,
			DontSupportRenameIndex:    true,
			DontSupportRenameColumn:   true,
			SkipInitializeWithVersion: false,
		}))
		if err != nil {
			panic(err)
		}
		db = gormDB
	}

	tableName := opts.TableName
	if len(tableName) == 0 {
		tableName = "sessions"
	}
	opts.TableName = tableName

	// test connection
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}

	if err = sqlDB.Ping(); err != nil {
		panic(err)
	}
	opts.GormDB = db
	return &Database{
		Options: opts,
		GormDB:  db,
	}
}

type Database struct {
	Options Options
	GormDB  *gorm.DB
	logger  *golog.Logger
}

// SetLogger sets the logger once before server ran.
// By default, the Iris one is injected.
func (db *Database) SetLogger(logger *golog.Logger) {
	db.logger = logger
}

// Acquire receives a session's lifetime from the database,
// if the return value is LifeTime{} then the session manager sets the life time based on the expiration duration lives in configuration.
func (db *Database) Acquire(sid string, expires time.Duration) sessions.LifeTime {
	var (
		isCreate   = false
		expireTime time.Time
	)

	err := db.GormDB.Transaction(func(tx *gorm.DB) error {
		s := db.getGormSessionBySid(tx, sid)
		if s != nil {
			expireTime = s.Expires
			return nil
		}
		cr := db.getDB(tx).Create(&GormSession{
			SessionKey:  sid,
			SessionData: &SessionData{},
			Expires:     time.Now().Add(expires),
		})
		if cr.Error != nil {
			return cr.Error
		}
		isCreate = true
		return nil
	})
	if err != nil {
		return sessions.LifeTime{} // session manager will handle the rest.
	}

	if isCreate {
		return sessions.LifeTime{}
	}
	return sessions.LifeTime{
		Time: expireTime,
	}
}

// OnUpdateExpiration will re-set the database's session's entry ttl.
func (db *Database) OnUpdateExpiration(sid string, newExpires time.Duration) error {
	ur := db.getDB(db.GormDB).Where(GormSession{SessionKey: sid}).
		Updates(GormSession{
			Expires: time.Now().Add(newExpires),
		})
	if ur.Error != nil {
		return errors.New("update expiration failed")
	}
	return nil
}

// Set sets a key value of a specific session.
// Ignore the "immutable".
func (db *Database) Set(sid string, key string, value interface{}, _ time.Duration, _ bool) error {
	return db.GormDB.Transaction(func(tx *gorm.DB) error {
		s := db.getGormSessionBySid(tx, sid)
		if s == nil {
			return errors.New("session not exist")
		}
		s.SessionData.Set(key, value)
		ur := db.getDB(tx).Where(GormSession{SessionKey: sid}).
			Updates(
				GormSession{
					SessionData: s.SessionData,
				})
		if ur.Error != nil {
			return errors.New("set session failed")
		}
		return nil
	})
}

// Get retrieves a session value based on the key.
func (db *Database) Get(sid string, key string) interface{} {
	s := db.getGormSessionBySid(db.GormDB, sid)
	if s == nil {
		return nil
	}
	data, err := s.SessionData.Get(key)
	if err != nil {
		return nil
	}
	return data
}

// Decode binds the "outPtr" to the value associated to the provided "key".
func (db *Database) Decode(sid, key string, outPtr interface{}) error {
	s := db.getGormSessionBySid(db.GormDB, sid)
	if s == nil {
		return errors.New("session not exist")
	}
	data, err := s.SessionData.Get(key)
	if err != nil {
		return err
	}
	return db.decodeValue(data, outPtr)
}

func (db *Database) decodeValue(val interface{}, outPtr interface{}) error {
	if val == nil {
		return nil
	}

	switch data := val.(type) {
	case []byte:
		// this is the most common type, as we save all values as []byte,
		// the only exception is where the value is string on HGetAll command.
		return sessions.DefaultTranscoder.Unmarshal(data, outPtr)
	case string:
		return sessions.DefaultTranscoder.Unmarshal([]byte(data), outPtr)
	default:
		return fmt.Errorf("unknown value type of %T", data)
	}
}

// Visit loops through all session keys and values.
func (db *Database) Visit(sid string, cb func(key string, value interface{})) error {
	s := db.getGormSessionBySid(db.GormDB, sid)
	if s == nil {
		return errors.New("session not exist")
	}
	s.SessionData.Visit(cb)
	return nil
}

// Len returns the length of the session's entries (keys).
func (db *Database) Len(sid string) int {
	s := db.getGormSessionBySid(db.GormDB, sid)
	if s == nil {
		return 0
	}
	return s.SessionData.Len()
}

// Delete removes a session key value based on its key.
func (db *Database) Delete(sid string, key string) bool {
	s := db.getGormSessionBySid(db.GormDB, sid)
	if s == nil {
		return false
	}
	s.SessionData.Delete(key)
	dr := db.getDB(db.GormDB).Where(GormSession{SessionKey: sid}).
		Updates(GormSession{
			SessionData: s.SessionData,
		})
	return dr.Error == nil
}

// Clear removes all session key values but it keeps the session entry.
func (db *Database) Clear(sid string) error {
	s := db.getGormSessionBySid(db.GormDB, sid)
	if s == nil {
		return errors.New("session not exist")
	}
	s.SessionData.Clear()
	ur := db.getDB(db.GormDB).Where(GormSession{SessionKey: sid}).
		Updates(GormSession{
			SessionData: s.SessionData,
		})
	return ur.Error
}

// Release destroys the session, it clears and removes the session entry,
// session manager will create a new session ID on the next request after this call.
func (db *Database) Release(sid string) error {
	dr := db.getDB(db.GormDB).Where(&GormSession{SessionKey: sid}).Delete(&GormSession{
		SessionKey: sid,
	})
	return dr.Error
}

// Close terminates the mysql connection.
func (db *Database) Close() error {
	// https://stackoverflow.com/questions/63816057/how-do-i-close-database-instance-in-gorm-1-20-0
	sqlDB, err := db.GormDB.DB()
	if err != nil {
		return err
	}
	if err = sqlDB.Close(); err != nil {
		return err
	}
	return nil
}

func (db *Database) getDB(gormDB *gorm.DB) *gorm.DB {
	gDB := gormDB
	if gDB == nil {
		gDB = db.GormDB
	}
	return gDB.Table(db.Options.TableName)
}

func (db *Database) getGormSessionBySid(gormDB *gorm.DB, sid string) *GormSession {
	s := &GormSession{}
	gDB := db.getDB(gormDB)
	sr := gDB.Table(db.Options.TableName).Where(GormSession{
		SessionKey: sid,
	}).Limit(1).Find(s)
	if sr.Error != nil || sr.RowsAffected == 0 {
		return nil
	}
	return s
}

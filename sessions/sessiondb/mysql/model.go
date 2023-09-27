package mysql

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type GormSession struct {
	ID          int64          `gorm:"column:id;primaryKey"`
	SessionKey  string         `gorm:"column:session_key"`
	SessionData *SessionData   `gorm:"column:session_data"`
	Expires     time.Time      `gorm:"column:expires"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at"`
}

type SessionData struct {
	data map[string]interface{}
}

func (sd *SessionData) GormDataType() string {
	return "text"
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (sd *SessionData) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal SessionData value:", value))
	}
	return sd.UnmarshalJSON(bytes)
}

// Value return json value, implement driver.Valuer interface
func (sd *SessionData) Value() (driver.Value, error) {
	if len(sd.data) == 0 {
		return []byte("{}"), nil
	}
	return sd.MarshalJSON()
}

// MarshalJSON to output non base64 encoded []byte
func (sd *SessionData) MarshalJSON() ([]byte, error) {
	return json.Marshal(sd.data)
}

// UnmarshalJSON to deserialize []byte
func (sd *SessionData) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &sd.data)
}

func (sd *SessionData) Get(key string) (interface{}, error) {
	value, exist := sd.data[key]
	if !exist {
		return "", errors.New(fmt.Sprintf("key %s not exist", key))
	}
	return value, nil
}

func (sd *SessionData) Set(key string, value interface{}) {
	if sd.data == nil {
		sd.data = make(map[string]interface{})
	}
	sd.data[key] = value
}

func (sd *SessionData) Delete(key string) {
	delete(sd.data, key)
}

func (sd *SessionData) Clear() {
	sd.data = make(map[string]interface{})
}

func (sd *SessionData) Visit(cb func(key string, value interface{})) {
	for key, value := range sd.data {
		cb(key, value)
	}
}

func (sd *SessionData) Len() int {
	return len(sd.data)
}

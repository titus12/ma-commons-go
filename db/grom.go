package db

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"strings"
)

var (
	gormDB *gorm.DB
)

// support pg and mysql
func InitGorm(addr string, maxIdle, maxOpen int) (err error) {
	attrs := strings.Split(addr, ":")
	var connector string
	if attrs[0] == "postgresql" {
		connector = "postgres"
	} else {
		connector = "mysql"
	}

	gormDB, err = gorm.Open(connector, addr)
	if err != nil {
		return
	}

	gormDB.LogMode(false)
	gormDB.SingularTable(true)

	gormDB.DB().SetMaxIdleConns(maxIdle)
	gormDB.DB().SetMaxOpenConns(maxOpen)
	gormDB.DB().Ping()

	return
}

// custom needs with the db conn
func GormDB() *gorm.DB {
	return gormDB
}

// common method ...
func Upsert(object interface{}, fields, wheres map[string]interface{}) error {
	return gormDB.Where(wheres).Assign(fields).FirstOrCreate(object).Error
}

func SelectOne(object interface{}, wheres map[string]interface{}) error {
	return gormDB.Where(wheres).First(object).Error
}

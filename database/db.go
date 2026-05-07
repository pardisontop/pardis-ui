package database

import (
	"bytes"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/pardisontop/pardis-ui/config"
	"github.com/pardisontop/pardis-ui/database/model"
	"github.com/pardisontop/pardis-ui/util/common"
	"github.com/pardisontop/pardis-ui/xray"

	"golang.org/x/crypto/bcrypt"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB
var currentDBType config.DBType

func initUser() error {
	err := db.AutoMigrate(&model.User{})
	if err != nil {
		return err
	}
	var count int64
	err = db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		hashedPwd, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user := &model.User{
			Username: "admin",
			Password: string(hashedPwd),
		}
		return db.Create(user).Error
	}
	return nil
}

func initInbound() error {
	return db.AutoMigrate(&model.Inbound{})
}

func initSetting() error {
	return db.AutoMigrate(&model.Setting{})
}

func initSubAccount() error {
	return db.AutoMigrate(&model.SubAccount{})
}

func initClientAnalytics() error {
	return db.AutoMigrate(&model.ClientConnectionSession{}, &model.ClientUsageSample{}, &model.ClientAppUsage{})
}

func initClientTraffic() error {
	return db.AutoMigrate(&xray.ClientTraffic{})
}

func getGormConfig() *gorm.Config {
	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	return &gorm.Config{
		Logger: gormLogger,
	}
}

func buildMySQLDSN(dbConfig config.DBConfig) (string, error) {
	dbConfig = dbConfig.Normalized()
	if dbConfig.DSN != "" {
		return dbConfig.DSN, nil
	}
	if dbConfig.Host == "" {
		return "", common.NewError("database host cannot be empty")
	}
	if dbConfig.Name == "" {
		return "", common.NewError("database name cannot be empty")
	}
	if dbConfig.Port <= 0 || dbConfig.Port > 65535 {
		return "", common.NewError("database port is not valid:", dbConfig.Port)
	}

	params, err := url.ParseQuery(dbConfig.Params)
	if err != nil {
		return "", err
	}
	if _, ok := params["charset"]; !ok {
		params.Set("charset", "utf8mb4")
	}

	parseTime := true
	if values, ok := params["parseTime"]; ok && len(values) > 0 {
		parseTime, err = strconv.ParseBool(values[0])
		if err != nil {
			return "", err
		}
		delete(params, "parseTime")
	}

	location := time.Local
	if values, ok := params["loc"]; ok && len(values) > 0 {
		location, err = time.LoadLocation(values[0])
		if err != nil {
			return "", err
		}
		delete(params, "loc")
	}

	flatParams := make(map[string]string, len(params))
	for key, values := range params {
		if len(values) == 0 {
			continue
		}
		flatParams[key] = values[len(values)-1]
	}

	driverConfig := mysqldriver.NewConfig()
	driverConfig.User = dbConfig.User
	driverConfig.Passwd = dbConfig.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(dbConfig.Host, strconv.Itoa(dbConfig.Port))
	driverConfig.DBName = dbConfig.Name
	driverConfig.ParseTime = parseTime
	driverConfig.Loc = location
	driverConfig.Params = flatParams

	return driverConfig.FormatDSN(), nil
}

func openDB(dbPath string, dbConfig config.DBConfig, gormConfig *gorm.Config) (*gorm.DB, config.DBType, error) {
	dbConfig = dbConfig.Normalized()
	if dbConfig.IsSQLite() {
		dir := path.Dir(dbPath)
		err := os.MkdirAll(dir, fs.ModeDir)
		if err != nil {
			return nil, "", err
		}
		gormDB, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
		return gormDB, config.DBTypeSQLite, err
	}

	if dbConfig.IsMySQLCompatible() {
		dsn, err := buildMySQLDSN(dbConfig)
		if err != nil {
			return nil, "", err
		}
		gormDB, err := gorm.Open(gormmysql.Open(dsn), gormConfig)
		return gormDB, dbConfig.Type, err
	}

	return nil, "", common.NewError("unsupported database type:", dbConfig.Type)
}

func InitDB(dbPath string) error {
	var err error
	db, currentDBType, err = openDB(dbPath, config.GetDBConfig(), getGormConfig())
	if err != nil {
		return err
	}

	err = initUser()
	if err != nil {
		return err
	}
	err = initInbound()
	if err != nil {
		return err
	}
	err = initSetting()
	if err != nil {
		return err
	}
	err = initSubAccount()
	if err != nil {
		return err
	}

	err = initClientTraffic()
	if err != nil {
		return err
	}
	err = initClientAnalytics()
	if err != nil {
		return err
	}

	return nil
}

func TestConfig(dbConfig config.DBConfig) error {
	gdb, _, err := openDB(config.GetDBPath(), dbConfig, &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return sqlDB.Ping()
}

func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsSQLite() bool {
	if currentDBType == "" {
		return config.GetDBConfig().IsSQLite()
	}
	return currentDBType == config.DBTypeSQLite
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func IsSQLiteDB(file io.Reader) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.Read(buf)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func Checkpoint() error {
	if !IsSQLite() || db == nil {
		return nil
	}
	// Update WAL
	err := db.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return err
	}
	return nil
}

func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil { // file must exist
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var res string
	if err := gdb.Raw("PRAGMA integrity_check;").Scan(&res).Error; err != nil {
		return err
	}
	if res != "ok" {
		return common.NewError("sqlite integrity check failed: " + res)
	}
	return nil
}

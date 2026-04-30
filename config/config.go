package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const dbFolderConfFile = "db_folder.conf"
const dbConfigFile = "db_config.json"

type DBType string

const (
	DBTypeSQLite        DBType = "sqlite"
	DBTypeMySQL         DBType = "mysql"
	DBTypeMariaDB       DBType = "mariadb"
	DBTypeMariaDBGalera DBType = "mariadb-galera"
)

type DBConfig struct {
	Type     DBType `json:"type" form:"type"`
	Host     string `json:"host" form:"host"`
	Port     int    `json:"port" form:"port"`
	Name     string `json:"name" form:"name"`
	User     string `json:"user" form:"user"`
	Password string `json:"password,omitempty" form:"password"`
	Params   string `json:"params" form:"params"`
	DSN      string `json:"dsn,omitempty" form:"dsn"`
}

func DefaultDBConfig() DBConfig {
	return DBConfig{
		Type:   DBTypeSQLite,
		Host:   "127.0.0.1",
		Port:   3306,
		Name:   "pardis_ui",
		Params: "charset=utf8mb4&parseTime=True&loc=Local",
	}
}

func (c DBConfig) Normalized() DBConfig {
	c.Type = DBType(strings.ToLower(strings.TrimSpace(string(c.Type))))
	if c.Type == "" {
		c.Type = DBTypeSQLite
	}
	c.Host = strings.TrimSpace(c.Host)
	c.Name = strings.TrimSpace(c.Name)
	c.User = strings.TrimSpace(c.User)
	c.Params = strings.TrimSpace(c.Params)
	c.DSN = strings.TrimSpace(c.DSN)
	if c.Port == 0 {
		c.Port = 3306
	}
	return c
}

func (c DBConfig) IsSQLite() bool {
	return c.Normalized().Type == DBTypeSQLite
}

func (c DBConfig) IsMySQLCompatible() bool {
	switch c.Normalized().Type {
	case DBTypeMySQL, DBTypeMariaDB, DBTypeMariaDBGalera:
		return true
	default:
		return false
	}
}

func (c DBConfig) Sanitized() DBConfig {
	c.Password = ""
	c.DSN = ""
	return c
}

//go:embed version
var version string

//go:embed name
var name string

type LogLevel string

const (
	Debug   LogLevel = "debug"
	Info    LogLevel = "info"
	Warning LogLevel = "warning"
	Error   LogLevel = "error"
)

func GetVersion() string {
	return strings.TrimSpace(version)
}

func GetName() string {
	return strings.TrimSpace(name)
}

func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("PARDIS_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return os.Getenv("PARDIS_DEBUG") == "true"
}

func GetBinFolderPath() string {
	binFolderPath := os.Getenv("PARDIS_BIN_FOLDER")
	if binFolderPath == "" {
		binFolderPath = "bin"
	}
	return binFolderPath
}

func getBaseDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	exeDir := filepath.Dir(exePath)
	exeDirLower := strings.ToLower(filepath.ToSlash(exeDir))
	if strings.Contains(exeDirLower, "/appdata/local/temp/") || strings.Contains(exeDirLower, "/go-build") {
		wd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return wd
	}
	return exeDir
}

func getDBFolderConfPath() string {
	return filepath.Join(getBaseDir(), dbFolderConfFile)
}

func getDBConfigPath() string {
	return filepath.Join(getBaseDir(), dbConfigFile)
}

func getDBFolderFromConf() string {
	data, err := os.ReadFile(getDBFolderConfPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func SetDBFolderPath(folderPath string) error {
	return os.WriteFile(getDBFolderConfPath(), []byte(folderPath), 0644)
}

func getDBConfigFromEnv() (DBConfig, bool) {
	typeValue := strings.TrimSpace(os.Getenv("PARDIS_DB_TYPE"))
	dsn := strings.TrimSpace(os.Getenv("PARDIS_DB_DSN"))
	if typeValue == "" && dsn == "" {
		return DBConfig{}, false
	}

	cfg := DefaultDBConfig()
	if typeValue != "" {
		cfg.Type = DBType(typeValue)
	} else {
		cfg.Type = DBTypeMySQL
	}
	if host := os.Getenv("PARDIS_DB_HOST"); host != "" {
		cfg.Host = host
	}
	if port := os.Getenv("PARDIS_DB_PORT"); port != "" {
		if parsedPort, err := strconv.Atoi(port); err == nil {
			cfg.Port = parsedPort
		}
	}
	if name := os.Getenv("PARDIS_DB_NAME"); name != "" {
		cfg.Name = name
	}
	if user := os.Getenv("PARDIS_DB_USER"); user != "" {
		cfg.User = user
	}
	if password := os.Getenv("PARDIS_DB_PASSWORD"); password != "" {
		cfg.Password = password
	}
	if params := os.Getenv("PARDIS_DB_PARAMS"); params != "" {
		cfg.Params = params
	}
	cfg.DSN = dsn

	return cfg.Normalized(), true
}

func IsDBConfigFromEnv() bool {
	_, ok := getDBConfigFromEnv()
	return ok
}

func GetDBConfig() DBConfig {
	// Priority: env vars > config file > SQLite default.
	if cfg, ok := getDBConfigFromEnv(); ok {
		return cfg
	}

	data, err := os.ReadFile(getDBConfigPath())
	if err != nil {
		return DefaultDBConfig()
	}

	cfg := DefaultDBConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultDBConfig()
	}
	return cfg.Normalized()
}

func SetDBConfig(cfg DBConfig) error {
	cfg = cfg.Normalized()
	if cfg.IsSQLite() {
		cfg = DefaultDBConfig()
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getDBConfigPath(), data, 0600)
}

func GetDBFolderPath() string {
	// Priority: env var > config file > platform default
	if envPath := os.Getenv("PARDIS_DB_FOLDER"); envPath != "" {
		return envPath
	}
	if confPath := getDBFolderFromConf(); confPath != "" {
		return confPath
	}
	if runtime.GOOS == "windows" {
		return getBaseDir()
	}
	return "/etc/pardis-ui"
}

func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

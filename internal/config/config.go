package config

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/goccy/go-yaml"
)

const (
	ConfigDir        = "./config/"
	ConfigFileFormat = "config-%s.yaml"
)

const (
	EnvProd = "prod"
	EnvTest = "test"
	EnvDev  = "dev"
)

const (
	RunModeMixed    = "mixed"
	RunModeAPI      = "api"
	RunModeAdminAPI = "admin_api"
)

type AppConfig struct {
	Name    string `yaml:"name"`
	Env     string `yaml:"env"`
	Debug   bool   `yaml:"debug"`
	Version string `yaml:"version"`
	FileUrl string `yaml:"file_url"`
	Mode    string `yaml:"mode"`
}

type Database struct {
	Dsn    string `yaml:"dsn"`
	Driver string `yaml:"driver"`
}

type Redis struct {
	Addr     string `yaml:"addr"` //127.0.0.1:6379
	Password string `yaml:"password"`
}

type JwtConfig struct {
	Secret        string `yaml:"secret"`
	ExpireSeconds int64  `yaml:"expire_seconds"`
}

type TOTPConfig struct {
	Enable bool   `yaml:"enable"`
	Issuer string `yaml:"issuer"`
}

type SystemConfig struct {
	UseStrictAuth bool `yaml:"use-strict-auth"`
}

type ApiDoc struct {
	Enable                  bool     `yaml:"enable"`                     //是否开启文档功能
	GenScanDir              []string `yaml:"gen-scan-dir"`               // 生成文档时扫描目录,常见如dto,model,handler等声明接口函数和对象的地方
	OutputSwaggerFile       string   `yaml:"output-swagger-file"`        // 兼容旧配置
	ApiOutputSwaggerFile    string   `yaml:"api-output-swagger-file"`    // 前端 API 文档输出文件
	AdminOutputSwaggerFile  string   `yaml:"admin-output-swagger-file"`  // 后台 Admin API 文档输出文件
	SystemOutputSwaggerFile string   `yaml:"system-output-swagger-file"` // 系统管理 API 文档输出文件
}

type ServerNode struct {
	Port string `yaml:"port"`
}

type ServersConfig struct {
	Mixed    ServerNode `yaml:"mixed"`
	API      ServerNode `yaml:"api"`
	AdminAPI ServerNode `yaml:"admin_api"`
}

// 服务端配置
type ServiceConfig struct {
	App         AppConfig     `yaml:"app"`
	Servers     ServersConfig `yaml:"servers"`
	ServiceName string        `yaml:"service_name"`
	IP          string        `yaml:"ip"`
	Port        string        `yaml:"port"`
	Debug       bool          `yaml:"debug"`
	Env         string        `yaml:"env"`
	Version     string        `yaml:"version"`
	FileUrl     string        `yaml:"file_url"`
	Database    Database      `yaml:"database"`
	Jwt         JwtConfig     `yaml:"jwt"`
	Redis       Redis         `yaml:"redis"`
	TOTP        TOTPConfig    `yaml:"totp"`
	System      SystemConfig  `yaml:"system"`
	ApiDoc      ApiDoc        `yaml:"api-doc"`
}

func (config *ServiceConfig) GetRunMode() string {
	mode := config.App.Mode
	if mode == "" {
		mode = RunModeMixed
	}
	switch mode {
	case RunModeMixed, RunModeAPI, RunModeAdminAPI:
		return mode
	default:
		return RunModeMixed
	}
}

func (config *ServiceConfig) ActiveServer() *ServerNode {
	switch config.GetRunMode() {
	case RunModeAPI:
		return &config.Servers.API
	case RunModeAdminAPI:
		return &config.Servers.AdminAPI
	default:
		return &config.Servers.Mixed
	}
}

func (config *ServiceConfig) ActiveListenAddr() string {
	server := config.ActiveServer()
	port := server.Port
	if port == "" {
		port = defaultPortByMode(config.GetRunMode())
	}
	return "0.0.0.0:" + port
}

func (config *ServiceConfig) ActiveServiceName() string {
	if config.App.Name == "" {
		return "shop-api-" + config.GetRunMode()
	}
	return config.App.Name + "-" + config.GetRunMode()
}

func (config *ServiceConfig) IsProdEnv() bool {
	if config.Env != EnvProd {
		return false
	}
	return true
}

func (config *ServiceConfig) IsDevEnv() bool {
	if config.Env != EnvDev {
		return false
	}
	return true
}

func (config *ServiceConfig) IsDebug() bool {
	if config.IsProdEnv() {
		return false
	}
	if config.Debug == true {
		return true
	}
	return false
}

var config = new(ServiceConfig)

func GetConfig() *ServiceConfig {
	return config
}

func GetConfigPath(env string) string {
	return path.Join(ConfigDir, fmt.Sprintf(ConfigFileFormat, env))
}

func InitConfig(path string, mode string) *ServiceConfig {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("InitConfig: ", path, ".  config err: ", err.Error())
		return nil
	}

	if err = yaml.Unmarshal(content, config); err != nil {
		log.Fatal("InitConfig: ", path, ".  config err: ", err.Error())
		return nil
	}
	config.normalize(mode)
	return config
}

func (config *ServiceConfig) normalize(mode string) {
	if config.App.Env == "" {
		config.App.Env = config.Env
	}
	if config.App.Version == "" {
		config.App.Version = config.Version
	}
	if config.App.FileUrl == "" {
		config.App.FileUrl = config.FileUrl
	}
	if config.App.Name == "" {
		config.App.Name = config.ServiceName
	}
	if config.App.Mode == "" {
		config.App.Mode = RunModeMixed
	}
	if mode != "" {
		config.App.Mode = mode
	}

	config.applyServerDefaults(&config.Servers.Mixed, RunModeMixed)
	config.applyServerDefaults(&config.Servers.API, RunModeAPI)
	config.applyServerDefaults(&config.Servers.AdminAPI, RunModeAdminAPI)

	active := config.ActiveServer()
	config.Env = config.App.Env
	config.Debug = config.App.Debug
	config.Version = config.App.Version
	config.FileUrl = config.App.FileUrl
	config.ServiceName = config.ActiveServiceName()
	config.IP = "0.0.0.0"
	config.Port = active.Port
	if config.Port == "" {
		config.Port = defaultPortByMode(config.GetRunMode())
	}
}

func (config *ServiceConfig) applyServerDefaults(node *ServerNode, mode string) {
	if node.Port == "" {
		node.Port = defaultPortByMode(mode)
	}
}

func defaultPortByMode(mode string) string {
	switch mode {
	case RunModeAPI:
		return "8801"
	case RunModeAdminAPI:
		return "8802"
	default:
		return "8800"
	}
}

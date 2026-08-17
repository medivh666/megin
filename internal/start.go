package internal

import "megin/pkg/logger"

// 服务启动后，初始化相关操作可以放在这里执行,比如全局变量，订阅kafka
func OnServerStart() error {
	logger.Info("OnServerStart Run....")
	return nil
}

package schedule

import (
	"megin/pkg/logger"
	"runtime"
	"runtime/debug"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

func goroutineWatcher() {
	num := runtime.NumGoroutine()
	logger.Info("NumGoroutine", zap.Int("num", num))
}

func Start() {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("schedule defer Error", zap.Any("err", err))
			debug.PrintStack()
		}
	}()

	schedule := cron.New(cron.WithSeconds())
	schedule.AddFunc("@every 1m", goroutineWatcher)
	schedule.Start()
}

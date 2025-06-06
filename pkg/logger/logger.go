package logger

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// New 创建新的日志实例
func New() *logrus.Logger {
	logger := logrus.New()
	
	// 设置日志级别
	level, err := logrus.ParseLevel(viper.GetString("logging.level"))
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// 设置日志格式
	if viper.GetString("logging.format") == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}

	// 设置输出
	if viper.GetString("logging.output") == "file" {
		logFile := viper.GetString("logging.file.path")
		if logFile == "" {
			logFile = "./logs/platform.log"
		}

		// 确保日志目录存在
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Errorf("创建日志目录失败: %v", err)
		}

		// 使用lumberjack进行日志轮转
		logger.SetOutput(&lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    viper.GetInt("logging.file.max_size"),    // MB
			MaxBackups: viper.GetInt("logging.file.max_backups"),
			MaxAge:     viper.GetInt("logging.file.max_age"),     // days
			Compress:   true,
		})
	} else {
		logger.SetOutput(os.Stdout)
	}

	return logger
}

// WithFields 创建带字段的日志条目
func WithFields(logger *logrus.Logger, fields map[string]interface{}) *logrus.Entry {
	return logger.WithFields(fields)
}
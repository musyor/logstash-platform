package logger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupViper(config map[string]interface{}) {
	viper.Reset()
	for key, value := range config {
		viper.Set(key, value)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		checkFunc   func(*testing.T, *logrus.Logger)
		cleanupFunc func()
	}{
		{
			name: "default configuration",
			config: map[string]interface{}{},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				assert.Equal(t, logrus.InfoLevel, logger.Level)
				assert.IsType(t, &logrus.TextFormatter{}, logger.Formatter)
				assert.Equal(t, os.Stdout, logger.Out)
			},
		},
		{
			name: "debug level with json format",
			config: map[string]interface{}{
				"logging.level":  "debug",
				"logging.format": "json",
			},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				assert.Equal(t, logrus.DebugLevel, logger.Level)
				assert.IsType(t, &logrus.JSONFormatter{}, logger.Formatter)
			},
		},
		{
			name: "invalid log level defaults to info",
			config: map[string]interface{}{
				"logging.level": "invalid",
			},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				assert.Equal(t, logrus.InfoLevel, logger.Level)
			},
		},
		{
			name: "file output with custom path",
			config: map[string]interface{}{
				"logging.output":             "file",
				"logging.file.path":          "./test-logs/test.log",
				"logging.file.max_size":      10,
				"logging.file.max_backups":   3,
				"logging.file.max_age":       7,
			},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				// Check that output is not stdout (it's a file writer)
				assert.NotEqual(t, os.Stdout, logger.Out)
				// Check log directory was created
				assert.DirExists(t, "./test-logs")
			},
			cleanupFunc: func() {
				os.RemoveAll("./test-logs")
			},
		},
		{
			name: "file output with default path",
			config: map[string]interface{}{
				"logging.output": "file",
			},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				assert.NotEqual(t, os.Stdout, logger.Out)
				assert.DirExists(t, "./logs")
			},
			cleanupFunc: func() {
				os.RemoveAll("./logs")
			},
		},
		{
			name: "text formatter configuration",
			config: map[string]interface{}{
				"logging.format": "text",
			},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				formatter, ok := logger.Formatter.(*logrus.TextFormatter)
				assert.True(t, ok)
				assert.Equal(t, "2006-01-02 15:04:05", formatter.TimestampFormat)
				assert.True(t, formatter.FullTimestamp)
			},
		},
		{
			name: "json formatter configuration",
			config: map[string]interface{}{
				"logging.format": "json",
			},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				formatter, ok := logger.Formatter.(*logrus.JSONFormatter)
				assert.True(t, ok)
				assert.Equal(t, "2006-01-02 15:04:05", formatter.TimestampFormat)
			},
		},
		{
			name: "error level configuration",
			config: map[string]interface{}{
				"logging.level": "error",
			},
			checkFunc: func(t *testing.T, logger *logrus.Logger) {
				assert.Equal(t, logrus.ErrorLevel, logger.Level)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			setupViper(tt.config)
			
			// Execute
			logger := New()
			
			// Assert
			require.NotNil(t, logger)
			tt.checkFunc(t, logger)
			
			// Cleanup
			if tt.cleanupFunc != nil {
				tt.cleanupFunc()
			}
		})
	}
}

func TestWithFields(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name   string
		fields map[string]interface{}
		check  func(*testing.T, *logrus.Entry)
	}{
		{
			name: "single field",
			fields: map[string]interface{}{
				"key": "value",
			},
			check: func(t *testing.T, entry *logrus.Entry) {
				assert.Equal(t, "value", entry.Data["key"])
			},
		},
		{
			name: "multiple fields",
			fields: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
				"key3": true,
			},
			check: func(t *testing.T, entry *logrus.Entry) {
				assert.Equal(t, "value1", entry.Data["key1"])
				assert.Equal(t, 123, entry.Data["key2"])
				assert.Equal(t, true, entry.Data["key3"])
			},
		},
		{
			name:   "empty fields",
			fields: map[string]interface{}{},
			check: func(t *testing.T, entry *logrus.Entry) {
				assert.Empty(t, entry.Data)
			},
		},
		{
			name:   "nil fields",
			fields: nil,
			check: func(t *testing.T, entry *logrus.Entry) {
				assert.Empty(t, entry.Data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := WithFields(logger, tt.fields)
			assert.NotNil(t, entry)
			assert.Equal(t, logger, entry.Logger)
			tt.check(t, entry)
		})
	}
}

func TestLoggerOutput(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		logFunc   func(*logrus.Logger)
		checkLog  func(*testing.T, string)
		cleanup   func()
	}{
		{
			name: "info level logs info and above",
			config: map[string]interface{}{
				"logging.level": "info",
			},
			logFunc: func(logger *logrus.Logger) {
				logger.Debug("debug message")
				logger.Info("info message")
				logger.Error("error message")
			},
			checkLog: func(t *testing.T, output string) {
				assert.NotContains(t, output, "debug message")
				assert.Contains(t, output, "info message")
				assert.Contains(t, output, "error message")
			},
		},
		{
			name: "json format output",
			config: map[string]interface{}{
				"logging.format": "json",
			},
			logFunc: func(logger *logrus.Logger) {
				logger.WithField("test", "value").Info("test message")
			},
			checkLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `"level":"info"`)
				assert.Contains(t, output, `"msg":"test message"`)
				assert.Contains(t, output, `"test":"value"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			setupViper(tt.config)
			
			// Capture output
			var buf bytes.Buffer
			logger := New()
			logger.SetOutput(&buf)
			
			// Execute
			tt.logFunc(logger)
			
			// Assert
			output := buf.String()
			tt.checkLog(t, output)
			
			// Cleanup
			if tt.cleanup != nil {
				tt.cleanup()
			}
		})
	}
}

func TestLoggerFileCreation(t *testing.T) {
	// Test that log directory creation error is handled
	t.Run("handle directory creation error", func(t *testing.T) {
		// Create a file where directory should be
		testFile := "./test-conflict/conflict.txt"
		os.MkdirAll(filepath.Dir(testFile), 0755)
		f, _ := os.Create(testFile)
		f.Close()
		defer os.RemoveAll("./test-conflict")
		
		// Try to create logger with conflicting path
		setupViper(map[string]interface{}{
			"logging.output":    "file",
			"logging.file.path": "./test-conflict/conflict.txt/logs/test.log",
		})
		
		// Capture stderr to check error message
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		
		logger := New()
		assert.NotNil(t, logger)
		
		w.Close()
		os.Stderr = oldStderr
		
		var buf bytes.Buffer
		io.Copy(&buf, r)
		// The error would be logged internally
	})
}

func TestLoggerIntegration(t *testing.T) {
	t.Run("full integration test", func(t *testing.T) {
		// Setup complex configuration
		logPath := "./test-integration/app.log"
		setupViper(map[string]interface{}{
			"logging.level":              "debug",
			"logging.format":             "json",
			"logging.output":             "file",
			"logging.file.path":          logPath,
			"logging.file.max_size":      1,
			"logging.file.max_backups":   2,
			"logging.file.max_age":       1,
		})
		defer os.RemoveAll("./test-integration")
		
		// Create logger
		logger := New()
		
		// Log messages with fields
		entry := WithFields(logger, map[string]interface{}{
			"component": "test",
			"action":    "integration",
		})
		
		entry.Debug("debug message")
		entry.Info("info message")
		entry.Warn("warn message")
		entry.Error("error message")
		
		// Verify file exists
		assert.FileExists(t, logPath)
		
		// Read and verify content
		content, err := os.ReadFile(logPath)
		require.NoError(t, err)
		
		logContent := string(content)
		assert.Contains(t, logContent, "debug message")
		assert.Contains(t, logContent, "component\":\"test")
		assert.Contains(t, logContent, "action\":\"integration")
	})
}
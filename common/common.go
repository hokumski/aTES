package common

import (
	"crypto/sha256"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"math/rand"
	"os"
	"strings"
	"time"
)

// HashSHA256 returns SHA256 of bytes
func HashSHA256(b []byte) string {
	h := sha256.New()
	h.Write(b)
	bs := h.Sum(nil)
	hash := fmt.Sprintf("%x", bs)
	return hash
}

// FromKeysAndValues produces map from list of keys and values
func FromKeysAndValues(vals ...interface{}) map[string]interface{} {
	r := map[string]interface{}{}
	if len(vals) >= 2 && len(vals)%2 == 0 {
		for i := 0; i < len(vals); i = i + 2 {
			r[fmt.Sprint(vals[i])] = vals[i+1]
		}
	}
	return r
}

const digitBytes = "0123456789"
const letterBytes = "abcdefghijklmnopqrstuwxyz"
const digitLetterBytes = digitBytes + letterBytes

// GenerateRandomString returns string containing N symbols: lowercase latins and digits
func GenerateRandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = digitLetterBytes[rand.Int63()%int64(len(digitLetterBytes))]
	}
	return string(b)
}

func GetNewEcho(logger *zap.SugaredLogger) *echo.Echo {
	e := echo.New()
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:      true,
		LogStatus:   true,
		LogRemoteIP: true,
		LogLatency:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logger.Infow("request",
				"date", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
				"url", v.URI,
				"status", v.Status,
				"ip", v.RemoteIP,
				"latency_human", v.Latency.String(),
			)
			return nil
		},
	}))
	return e
}

func GetZapCore(forDevel bool) zapcore.Core {
	allLevels := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		if forDevel {
			return true
		}
		return lvl > zapcore.DebugLevel
	})

	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

	var cores []zapcore.Core
	consoleOutput := zapcore.Lock(os.Stderr)
	stdoutSyncer := zapcore.AddSync(consoleOutput)
	cores = append(cores, zapcore.NewCore(consoleEncoder, stdoutSyncer, allLevels))

	return zapcore.NewTee(cores...)
}

func EnsureServerProtocol(server string) string {
	server = strings.ToLower(server)
	if strings.HasPrefix(server, "https://") || strings.HasPrefix(server, "http://") {
		return server
	}
	if strings.HasPrefix(server, "localhost") {
		return fmt.Sprintf("http://%s", server)
	}
	return fmt.Sprintf("https://%s", server)
}

package log

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type SyncFunc func() error

func GetLogger(level zapcore.Level) (*zap.SugaredLogger, SyncFunc, error) {
	logConfig := zap.NewProductionConfig()
	logConfig.Level = zap.NewAtomicLevelAt(level)
	logConfig.Encoding = "console"
	logConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := logConfig.Build()
	if err != nil {
		return nil, nil, errors.Wrap(err, "building logger from config failed")
	}

	return logger.Sugar(), logger.Sync, nil
}

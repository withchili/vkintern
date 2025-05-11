package main

import (
	"go.uber.org/zap"
)

func NewLogger() *zap.Logger {
	l, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	return l
}

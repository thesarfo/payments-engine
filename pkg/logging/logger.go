package logging

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

const (
	defaultServiceName = "payments-engine"
	defaultEnvironment = "development"
)

func New() zerolog.Logger {
	service := strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	if service == "" {
		service = defaultServiceName
	}

	environment := strings.TrimSpace(os.Getenv("APP_ENV"))
	if environment == "" {
		environment = defaultEnvironment
	}

	return zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", service).
		Str("env", environment).
		Logger()
}

package config

import (
	"fmt"
	"time"
)

type TimeoutsConfig struct {
	// http servers
	Shutdown  time.Duration `env:"TIMEOUTS_SHUTDOWN" envDefault:"10s"`
	HTTPRead  time.Duration `env:"TIMEOUTS_HTTP_READ" envDefault:"5s"`
	HTTPWrite time.Duration `env:"TIMEOUTS_HTTP_WRITE" envDefault:"10s"`
	HTTPIdle  time.Duration `env:"TIMEOUTS_HTTP_IDLE" envDefault:"60s"`
	// grpc servers
	GRPCMaxConnIdle time.Duration `env:"TIMEOUTS_GRPC_KEEPALIVE_MAX_CONNECTION_IDLE" envDefault:"5m"`
	GRPCMaxConnAge  time.Duration `env:"TIMEOUTS_GRPC_KEEPALIVE_MAX_CONNECTION_AGE" envDefault:"30m"`
	GRPCKeepAlive   time.Duration `env:"TIMEOUTS_GRPC_KEEPALIVE" envDefault:"10s"`
	// clients
	BotClientReq      time.Duration `env:"TIMEOUTS_BOT_CLIENT_REQUEST" envDefault:"1s"`
	ScrapperClientReq time.Duration `env:"TIMEOUTS_SCRAPPER_CLIENT_REQUEST" envDefault:"1s"`
}

func (t TimeoutsConfig) Validate(protocol ProtocolType) error {
	if t.Shutdown <= 0 {
		return fmt.Errorf("shutdown timeout: %q", t.Shutdown)
	}

	if t.BotClientReq <= 0 {
		return fmt.Errorf("bot client timeout: %q", t.BotClientReq)
	}

	if t.ScrapperClientReq <= 0 {
		return fmt.Errorf("scrapper client timeout: %q", t.ScrapperClientReq)
	}

	if protocol == ProtocolHTTP {
		if t.HTTPRead <= 0 {
			return fmt.Errorf("HTTP read timeout: %q", t.HTTPRead)
		}

		if t.HTTPWrite <= 0 {
			return fmt.Errorf("HTTP write timeout: %q", t.HTTPWrite)
		}

		if t.HTTPIdle <= 0 {
			return fmt.Errorf("HTTP idle timeout: %q", t.HTTPIdle)
		}
	}

	if protocol == ProtocolGRPC {
		if t.GRPCMaxConnIdle <= 0 {
			return fmt.Errorf("gRPC max connection idle timeout: %q", t.GRPCMaxConnIdle)
		}

		if t.GRPCMaxConnAge <= 0 {
			return fmt.Errorf("gRPC max connection age timeout: %q", t.GRPCMaxConnAge)
		}

		if t.GRPCKeepAlive <= 0 {
			return fmt.Errorf("gRPC keepalive timeout: %q", t.GRPCKeepAlive)
		}
	}

	return nil
}

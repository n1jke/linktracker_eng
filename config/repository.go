package config

import (
	"errors"
	"fmt"
	"time"
)

type AccessType string

const (
	AccessSQL     AccessType = "sql"
	AccessBuilder AccessType = "builder"
)

type DatabaseConfig struct {
	Host     string     `env:"DB_HOST,required,notEmpty"`
	Port     int        `env:"DB_PORT,required,notEmpty"`
	Username string     `env:"DB_USERNAME,required,notEmpty" json:"-" yaml:"-"`
	Password string     `env:"DB_PASSWORD,required,notEmpty" json:"-" yaml:"-"`
	Name     string     `env:"DB_NAME,required,notEmpty"`
	Access   AccessType `env:"DB_ACCESS_TYPE,required,notEmpty"`
}

func (d *DatabaseConfig) Validate() error {
	if d.Access != AccessBuilder && d.Access != AccessSQL {
		return fmt.Errorf("access type: %q", d.Access)
	}

	return nil
}

func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?target_session_attrs=read-write&sslmode=disable",
		d.Username,
		d.Password,
		d.Host,
		d.Port,
		d.Name,
	)
}

type ValkeyConfig struct {
	Endpoints  []string      `env:"VALKEY_ENDPOINTS" envSeparator:","`
	MasterName string        `env:"VALKEY_MASTER_NAME,required,notEmpty"`
	Username   string        `env:"VALKEY_USERNAME,required,notEmpty" json:"-" yaml:"-"`
	Password   string        `env:"VALKEY_PASSWORD,required,notEmpty" json:"-" yaml:"-"`
	TTL        time.Duration `env:"VALKEY_TTL,required,notEmpty"`
	Enabled    bool          `env:"VALKEY_ENABLED,required,notEmpty"`
}

func (v *ValkeyConfig) Validate() error {
	if !v.Enabled {
		return nil
	}

	if len(v.Endpoints) == 0 {
		return errors.New("endpoints empty")
	}

	if v.TTL <= 0 {
		return errors.New("invalid ttl")
	}

	return nil
}

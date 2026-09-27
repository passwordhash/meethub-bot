package app

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/passwordhash/cascade-hub-bot/internal/config"
)

var errDatabaseConfig = errors.New("could not initialize database connection configuration")
var errDatabaseConnection = errors.New("could not connect to database")

func initPostgresDB(ctx context.Context, cfg config.PostgresConfig) (*pgxpool.Pool, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:   "/" + cfg.Database,
	}
	query := dsn.Query()
	query.Set("sslmode", cfg.SSLMode)
	dsn.RawQuery = query.Encode()
	poolConfig, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return nil, errDatabaseConfig
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errDatabaseConfig
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errDatabaseConnection
	}
	return pool, nil
}

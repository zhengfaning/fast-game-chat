package repository

import (
    "context"
    "fmt"
    "log"

    "github.com/jackc/pgx/v4/pgxpool"
)

type Database struct {
    Pool *pgxpool.Pool
}

func NewDatabase(dsn string) (*Database, error) {
    config, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, fmt.Errorf("unable to parse database config: %w", err)
    }

    pool, err := pgxpool.ConnectConfig(context.Background(), config)
    if err != nil {
        return nil, fmt.Errorf("unable to connect to database: %w", err)
    }

    // Ping to verify connection
    if err := pool.Ping(context.Background()); err != nil {
        return nil, fmt.Errorf("unable to ping database: %w", err)
    }

    log.Println("Connected to PostgreSQL")
    return &Database{Pool: pool}, nil
}

func (db *Database) Close() {
    db.Pool.Close()
}

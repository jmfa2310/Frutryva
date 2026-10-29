package httpapi

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type Variant struct {
	SportCode string `json:"sportCode"`
	Sport     string `json:"sport"`
	Variant   string `json:"variant"`
	Players   int    `json:"playersOnField"`
}

type Store interface {
	Ready(context.Context) error
	Sports(context.Context) ([]Variant, error)
}

type database interface {
	Ping(context.Context) error
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// PostgresStore keeps SQL and row decoding outside HTTP handlers.
type PostgresStore struct{ db database }

func NewPostgresStore(db database) *PostgresStore { return &PostgresStore{db: db} }

func (s *PostgresStore) Ready(ctx context.Context) error {
	if err := s.db.Ping(ctx); err != nil {
		return err
	}
	// A reachable database without the migrated sports schema is not ready.
	rows, err := s.db.Query(ctx, `SELECT s.code, s.name, v.name, v.players_on_field FROM sports s JOIN sport_variants v ON v.sport_id=s.id LIMIT 1`)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.SportCode, &v.Sport, &v.Variant, &v.Players); err != nil {
			return err
		}
		found = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !found {
		return errors.New("sports catalog is not initialized")
	}
	return nil
}

func (s *PostgresStore) Sports(ctx context.Context) ([]Variant, error) {
	rows, err := s.db.Query(ctx, `SELECT s.code,s.name,v.name,v.players_on_field FROM sports s JOIN sport_variants v ON v.sport_id=s.id ORDER BY s.id,v.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Variant{}
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.SportCode, &v.Sport, &v.Variant, &v.Players); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	drugpb "drugsync/proto"
)

// ErrNotFound is returned when a drug id doesn't exist in the table.
var ErrNotFound = errors.New("drug not found")

// Store holds the database connection pool.
type Store struct {
	db *sql.DB
}

// New opens the connection and verifies the database is actually reachable.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close shuts down the pool. Called when the server stops.
func (s *Store) Close() error {
	return s.db.Close()
}

// ---------- WRITE ----------

const saveQuery = `
INSERT INTO drugs (id, brand_name, generic_name, manufacturer,
                   product_ndc, product_type, route, substance_name, synced_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
ON CONFLICT (id) DO UPDATE SET
	brand_name     = EXCLUDED.brand_name,
	generic_name   = EXCLUDED.generic_name,
	manufacturer   = EXCLUDED.manufacturer,
	product_ndc    = EXCLUDED.product_ndc,
	product_type   = EXCLUDED.product_type,
	route          = EXCLUDED.route,
	substance_name = EXCLUDED.substance_name,
	synced_at      = NOW()`

// Save inserts a drug, or updates it if that id is already stored.
func (s *Store) Save(ctx context.Context, d *drugpb.Drug) error {
	_, err := s.db.ExecContext(ctx, saveQuery,
		d.Id, d.BrandName, d.GenericName, d.Manufacturer,
		d.ProductNdc, d.ProductType, d.Route, d.SubstanceName)
	if err != nil {
		return fmt.Errorf("saving drug %s: %w", d.Id, err)
	}
	return nil
}

// ---------- READ ONE ----------

const columns = `id, brand_name, generic_name, manufacturer,
                 product_ndc, product_type, route, substance_name`

const getQuery = `SELECT ` + columns + ` FROM drugs WHERE id = $1`

// GetByID fetches a single drug. Returns ErrNotFound if the id isn't stored.
func (s *Store) GetByID(ctx context.Context, id string) (*drugpb.Drug, error) {
	d := &drugpb.Drug{}
	err := s.db.QueryRowContext(ctx, getQuery, id).Scan(
		&d.Id, &d.BrandName, &d.GenericName, &d.Manufacturer,
		&d.ProductNdc, &d.ProductType, &d.Route, &d.SubstanceName)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading drug %s: %w", id, err)
	}
	return d, nil
}

// ---------- READ MANY ----------

const searchQuery = `SELECT ` + columns + `
FROM drugs
WHERE LOWER(brand_name) LIKE LOWER($1)
ORDER BY brand_name
LIMIT $2`

// SearchByBrand finds drugs whose brand name contains the query text.
func (s *Store) SearchByBrand(ctx context.Context, query string, limit int32) ([]*drugpb.Drug, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, searchQuery, "%"+query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("searching for %q: %w", query, err)
	}
	defer rows.Close()

	var drugs []*drugpb.Drug
	for rows.Next() {
		d := &drugpb.Drug{}
		if err := rows.Scan(
			&d.Id, &d.BrandName, &d.GenericName, &d.Manufacturer,
			&d.ProductNdc, &d.ProductType, &d.Route, &d.SubstanceName); err != nil {
			return nil, fmt.Errorf("reading search row: %w", err)
		}
		drugs = append(drugs, d)
	}

	// rows.Err() reports a failure that happened mid-iteration —
	// the loop above ends silently on error without this check.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("finishing search: %w", err)
	}
	return drugs, nil
}
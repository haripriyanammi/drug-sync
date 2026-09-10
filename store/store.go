package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	drugpb "drugsync/proto"
)

var (
	ErrNotFound      = errors.New("drug not found")
	ErrAlreadyExists = errors.New("drug already exists")
	ErrNoFields      = errors.New("no fields to update")
)

const columns = `id, brand_name, generic_name, manufacturer,
                 product_ndc, product_type, route, substance_name`

type Store struct {
	db *sql.DB
}

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

func (s *Store) Close() error {
	return s.db.Close()
}

// scanDrug fills a Drug from one row. Used by every read.
func scanDrug(row interface{ Scan(...any) error }) (*drugpb.Drug, error) {
	d := &drugpb.Drug{}
	err := row.Scan(
		&d.Id, &d.BrandName, &d.GenericName, &d.Manufacturer,
		&d.ProductNdc, &d.ProductType, &d.Route, &d.SubstanceName)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// ---------- SYNC (upsert) ----------

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

const getQuery = `SELECT ` + columns + ` FROM drugs WHERE id = $1`

func (s *Store) GetByID(ctx context.Context, id string) (*drugpb.Drug, error) {
	d, err := scanDrug(s.db.QueryRowContext(ctx, getQuery, id))
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
		d, err := scanDrug(rows)
		if err != nil {
			return nil, fmt.Errorf("reading search row: %w", err)
		}
		drugs = append(drugs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("finishing search: %w", err)
	}
	return drugs, nil
}

// ---------- CREATE ----------

const createQuery = `
INSERT INTO drugs (id, brand_name, generic_name, manufacturer,
                   product_ndc, product_type, route, substance_name, synced_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
ON CONFLICT (id) DO NOTHING
RETURNING ` + columns

func (s *Store) Create(ctx context.Context, d *drugpb.Drug) (*drugpb.Drug, error) {
	created, err := scanDrug(s.db.QueryRowContext(ctx, createQuery,
		d.Id, d.BrandName, d.GenericName, d.Manufacturer,
		d.ProductNdc, d.ProductType, d.Route, d.SubstanceName))

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAlreadyExists
	}
	if err != nil {
		return nil, fmt.Errorf("creating drug %s: %w", d.Id, err)
	}
	return created, nil
}

// ---------- UPDATE (full replace) ----------

const updateQuery = `
UPDATE drugs SET
	brand_name     = $2,
	generic_name   = $3,
	manufacturer   = $4,
	product_ndc    = $5,
	product_type   = $6,
	route          = $7,
	substance_name = $8,
	synced_at      = NOW()
WHERE id = $1
RETURNING ` + columns

func (s *Store) Update(ctx context.Context, id string, d *drugpb.Drug) (*drugpb.Drug, error) {
	updated, err := scanDrug(s.db.QueryRowContext(ctx, updateQuery,
		id, d.BrandName, d.GenericName, d.Manufacturer,
		d.ProductNdc, d.ProductType, d.Route, d.SubstanceName))

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updating drug %s: %w", id, err)
	}
	return updated, nil
}

// ---------- PATCH (partial update) ----------

func (s *Store) Patch(ctx context.Context, id string, fields map[string]string) (*drugpb.Drug, error) {
	if len(fields) == 0 {
		return nil, ErrNoFields
	}

	allowed := map[string]bool{
		"brand_name": true, "generic_name": true, "manufacturer": true,
		"product_ndc": true, "product_type": true, "route": true,
		"substance_name": true,
	}

	setParts := []string{}
	args := []any{id}

	for column, value := range fields {
		if !allowed[column] {
			return nil, fmt.Errorf("unknown column %q", column)
		}
		args = append(args, value)
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	query := fmt.Sprintf(
		"UPDATE drugs SET %s, synced_at = NOW() WHERE id = $1 RETURNING %s",
		strings.Join(setParts, ", "), columns)

	patched, err := scanDrug(s.db.QueryRowContext(ctx, query, args...))

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("patching drug %s: %w", id, err)
	}
	return patched, nil
}

// ---------- DELETE ----------

const deleteQuery = `DELETE FROM drugs WHERE id = $1`

func (s *Store) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, deleteQuery, id)
	if err != nil {
		return fmt.Errorf("deleting drug %s: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

package repository

import (
	"database/sql"
	"fmt"

	"github.com/synapseai/platform/services/auth/internal/domain"
	_ "github.com/lib/pq"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(query, user.ID, user.Email, user.PasswordHash, user.Name, user.Role, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *UserRepository) FindByEmail(email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, name, role, created_at, updated_at
		FROM users WHERE email = $1
	`
	user := &domain.User{}
	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

func (r *UserRepository) FindByID(id string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, name, role, created_at, updated_at
		FROM users WHERE id = $1
	`
	user := &domain.User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

func (r *UserRepository) EmailExists(email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	err := r.db.QueryRow(query, email).Scan(&exists)
	return exists, err
}

// Initialize database schema
func (r *UserRepository) InitSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(36) PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			role VARCHAR(50) DEFAULT 'user',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

		CREATE TABLE IF NOT EXISTS refresh_tokens (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			token TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);
	`
	_, err := r.db.Exec(schema)
	return err
}

package repository

import (
    "database/sql"
    "github.com/google/uuid"
)

type User struct {
    ID       uuid.UUID `json:"id"`
    Username string    `json:"username"`
    Email    string    `json:"email"`
}

type UserRepo struct {
    db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
    return &UserRepo{db: db}
}

func (r *UserRepo) Create(u *User) error {
    u.ID = uuid.New()
    _, err := r.db.Exec(`INSERT INTO users (id, username, email) VALUES ($1, $2, $3)`,
        u.ID, u.Username, u.Email)
    return err
}

func (r *UserRepo) GetAll() ([]User, error) {
    rows, err := r.db.Query(`SELECT id, username, email FROM users`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var users []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Username, &u.Email); err != nil {
            return nil, err
        }
        users = append(users, u)
    }
    return users, rows.Err()
}

func (r *UserRepo) GetByID(id uuid.UUID) (*User, error) {
    u := &User{}
    err := r.db.QueryRow(`SELECT id, username, email FROM users WHERE id=$1`, id).
        Scan(&u.ID, &u.Username, &u.Email)
    if err != nil {
        return nil, err
    }
    return u, nil
}

func (r *UserRepo) Update(id uuid.UUID, u *User) error {
    _, err := r.db.Exec(`UPDATE users SET username=$1, email=$2 WHERE id=$3`,
        u.Username, u.Email, id)
    return err
}

func (r *UserRepo) Delete(id uuid.UUID) error {
    _, err := r.db.Exec(`DELETE FROM users WHERE id=$1`, id)
    return err
}
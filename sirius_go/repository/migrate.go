package repository

import "database/sql"

func Migrate(db *sql.DB) error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS users (
            id UUID PRIMARY KEY,
            username TEXT NOT NULL,
            email TEXT NOT NULL UNIQUE
        )`,
        `CREATE TABLE IF NOT EXISTS profiles (
            id UUID PRIMARY KEY,
            name TEXT NOT NULL,
            lname TEXT NOT NULL,
            user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE
        )`,
    }
    for _, q := range queries {
        if _, err := db.Exec(q); err != nil {
            return err
        }
    }
    return nil
}
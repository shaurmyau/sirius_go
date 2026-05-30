package repository

import (
    "database/sql"
    "github.com/google/uuid"
)

type Profile struct {
    ID     uuid.UUID `json:"id"`
    Name   string    `json:"name"`
    Lname  string    `json:"lname"`
    UserID uuid.UUID `json:"user_id"`
}

type ProfileRepo struct {
    db *sql.DB
}

func NewProfileRepo(db *sql.DB) *ProfileRepo {
    return &ProfileRepo{db: db}
}

func (r *ProfileRepo) Create(p *Profile) error {
    p.ID = uuid.New()
    _, err := r.db.Exec(`INSERT INTO profiles (id, name, lname, user_id) VALUES ($1, $2, $3, $4)`,
        p.ID, p.Name, p.Lname, p.UserID)
    return err
}

func (r *ProfileRepo) GetByUserID(userID uuid.UUID) (*Profile, error) {
    p := &Profile{}
    err := r.db.QueryRow(`SELECT id, name, lname, user_id FROM profiles WHERE user_id=$1`, userID).
        Scan(&p.ID, &p.Name, &p.Lname, &p.UserID)
    if err != nil {
        return nil, err
    }
    return p, nil
}

func (r *ProfileRepo) Update(userID uuid.UUID, p *Profile) error {
    _, err := r.db.Exec(`UPDATE profiles SET name=$1, lname=$2 WHERE user_id=$3`,
        p.Name, p.Lname, userID)
    return err
}

func (r *ProfileRepo) Delete(userID uuid.UUID) error {
    _, err := r.db.Exec(`DELETE FROM profiles WHERE user_id=$1`, userID)
    return err
}
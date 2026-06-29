package handler

import (
    "encoding/json"
    "net/http"

    "github.com/example/go-server/repository"
    "github.com/google/uuid"
)

type UserHandler struct {
    repo *repository.UserRepo
}

func NewUserHandler(repo *repository.UserRepo) *UserHandler {
    return &UserHandler{repo: repo}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
    users, err := h.repo.GetAll()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(users)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var u repository.User
    if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    if err := h.repo.Create(&u); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(u)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request, idStr string) {
    id, err := uuid.Parse(idStr)
    if err != nil {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }
    u, err := h.repo.GetByID(id)
    if err != nil {
        http.Error(w, "user not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(u)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request, idStr string) {
    id, err := uuid.Parse(idStr)
    if err != nil {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }
    var u repository.User
    if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    if err := h.repo.Update(id, &u); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    u.ID = id
    json.NewEncoder(w).Encode(u)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request, idStr string) {
    id, err := uuid.Parse(idStr)
    if err != nil {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }
    if err := h.repo.Delete(id); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
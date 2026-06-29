package handler

import (
    "encoding/json"
    "net/http"

    "github.com/example/go-server/middleware"
    "github.com/example/go-server/repository"
    "github.com/google/uuid"
)

type ProfileHandler struct {
    repo *repository.ProfileRepo
}

func NewProfileHandler(repo *repository.ProfileRepo) *ProfileHandler {
    return &ProfileHandler{repo: repo}
}

// userIDFromCtx извлекает UUID пользователя из контекста, сохранённого JWT-мидлваре.
func userIDFromCtx(r *http.Request) uuid.UUID {
    return r.Context().Value(middleware.UserIDKey).(uuid.UUID)
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
    uid := userIDFromCtx(r)
    p, err := h.repo.GetByUserID(uid)
    if err != nil {
        http.Error(w, "profile not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(p)
}

func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
    uid := userIDFromCtx(r)
    // проверяем, нет ли уже профиля
    if _, err := h.repo.GetByUserID(uid); err == nil {
        http.Error(w, "profile already exists", http.StatusConflict)
        return
    }
    var p repository.Profile
    if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    p.UserID = uid
    if err := h.repo.Create(&p); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(p)
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
    uid := userIDFromCtx(r)
    var p repository.Profile
    if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    if err := h.repo.Update(uid, &p); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    // возвращаем обновлённый профиль
    updated, _ := h.repo.GetByUserID(uid)
    json.NewEncoder(w).Encode(updated)
}

func (h *ProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
    uid := userIDFromCtx(r)
    if err := h.repo.Delete(uid); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
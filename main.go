package main

import (
    "database/sql"
    "fmt"
    "log"
    "net/http"
    "os"

    _ "github.com/lib/pq"
    "github.com/example/go-server/handler"
    "github.com/example/go-server/middleware"
    "github.com/example/go-server/repository"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    dbHost := getEnv("DB_HOST", "localhost")
    dbPort := getEnv("DB_PORT", "5432")
    dbUser := getEnv("DB_USER", "postgres")
    dbPass := getEnv("DB_PASSWORD", "postgres")
    dbName := getEnv("DB_NAME", "goapp")
    jwtSecret := getEnv("JWT_SECRET", "mysecret")

    connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        dbHost, dbPort, dbUser, dbPass, dbName)
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatalf("DB open error: %v", err)
    }
    defer db.Close()
    if err = db.Ping(); err != nil {
        log.Fatalf("DB ping failed: %v", err)
    }

    if err := repository.Migrate(db); err != nil {
        log.Fatalf("Migration error: %v", err)
    }

    userRepo := repository.NewUserRepo(db)
    profileRepo := repository.NewProfileRepo(db)

    userHandler := handler.NewUserHandler(userRepo)
    profileHandler := handler.NewProfileHandler(profileRepo)

    mux := http.NewServeMux()

    // User CRUD без аутентификации
    mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            userHandler.List(w, r)
        case http.MethodPost:
            userHandler.Create(w, r)
        default:
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        }
    })
    mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
        id := r.URL.Path[len("/api/users/"):]
        if id == "" {
            http.NotFound(w, r)
            return
        }
        switch r.Method {
        case http.MethodGet:
            userHandler.Get(w, r, id)
        case http.MethodPut:
            userHandler.Update(w, r, id)
        case http.MethodDelete:
            userHandler.Delete(w, r, id)
        default:
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        }
    })

    // Profile CRUD – требуется JWT
    jwtMW := middleware.JWTAuth(jwtSecret)
    profileMux := http.NewServeMux()
    profileMux.HandleFunc("/api/profile", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            profileHandler.Get(w, r)
        case http.MethodPost:
            profileHandler.Create(w, r)
        case http.MethodPut:
            profileHandler.Update(w, r)
        case http.MethodDelete:
            profileHandler.Delete(w, r)
        default:
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        }
    })
    mux.Handle("/api/profile", jwtMW(profileMux))
    mux.Handle("/api/profile/", jwtMW(profileMux))

    // Метрики
    mux.Handle("/metrics", promhttp.Handler())

    // Оборачиваем всё в метрики‑middleware
    metricsMW := middleware.MetricsMiddleware()
    handler := metricsMW(mux)

    port := getEnv("PORT", "8080")
    log.Printf("Server listening on :%s", port)
    log.Fatal(http.ListenAndServe(":"+port, handler))
}

func getEnv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
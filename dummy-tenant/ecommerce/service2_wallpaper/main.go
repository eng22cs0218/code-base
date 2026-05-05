package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    _ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type Wallpaper struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    ImageURL string `json:"image_url"`
}

func main() {
    var err error
    db, err = sql.Open("sqlite3", "./wallpaper.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    if err := createTable(); err != nil {
        log.Fatal(err)
    }

    if err := seedData(); err != nil {
        log.Fatal(err)
    }

    http.HandleFunc("/health", healthHandler)
    http.HandleFunc("/wallpapers", withCORS(getWallpapers))

    fmt.Println("Wallpaper Service running on port 8082...")
    log.Fatal(http.ListenAndServe(":8082", nil))
}

func createTable() error {
    query := `
    CREATE TABLE IF NOT EXISTS wallpapers (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT,
        image_url TEXT
    );`
    _, err := db.Exec(query)
    return err
}

func seedData() error {
    var count int
    db.QueryRow("SELECT COUNT(*) FROM wallpapers").Scan(&count)
    if count > 0 {
        return nil
    }

    wallpapers := []Wallpaper{
        {Name: "Nature", ImageURL: "https://picsum.photos/id/1018/600/400"},
        {Name: "Mountains", ImageURL: "https://picsum.photos/id/1016/600/400"},
        {Name: "Ocean", ImageURL: "https://picsum.photos/id/1015/600/400"},
        {Name: "City", ImageURL: "https://picsum.photos/id/1020/600/400"},
        {Name: "Forest", ImageURL: "https://picsum.photos/id/1024/600/400"},
    }

    for _, w := range wallpapers {
        _, err := db.Exec("INSERT INTO wallpapers (name, image_url) VALUES (?, ?)", w.Name, w.ImageURL)
        if err != nil {
            return err
        }
    }
    return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func getWallpapers(w http.ResponseWriter, r *http.Request) {
    rows, err := db.Query("SELECT id, name, image_url FROM wallpapers")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var wallpapers []Wallpaper
    for rows.Next() {
        var w Wallpaper
        rows.Scan(&w.ID, &w.Name, &w.ImageURL)
        wallpapers = append(wallpapers, w)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(wallpapers)
}

func withCORS(handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusOK)
            return
        }
        handler(w, r)
    }
}

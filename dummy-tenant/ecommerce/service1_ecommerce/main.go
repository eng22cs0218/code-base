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

type CartItem struct {
    ID          int     `json:"id"`
    ProductName string  `json:"product_name"`
    Quantity    int     `json:"quantity"`
    Price       float64 `json:"price"`
}

func main() {
    var err error
    db, err = sql.Open("sqlite3", "./ecommerce.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    if err := createTable(); err != nil {
        log.Fatal(err)
    }

    http.HandleFunc("/health", withCORS(healthHandler))
    http.HandleFunc("/add-cart", withCORS(addToCart))
    http.HandleFunc("/update-cart", withCORS(updateCart))
    http.HandleFunc("/remove-cart", withCORS(removeFromCart))
    http.HandleFunc("/view-cart", withCORS(viewCart))
    http.HandleFunc("/checkout", withCORS(checkout))

    fmt.Println("E-commerce Service running on port 8081...")
    log.Fatal(http.ListenAndServe(":8081", nil))
}

func createTable() error {
    query := `
    CREATE TABLE IF NOT EXISTS cart_items (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        product_name TEXT UNIQUE,
        quantity INTEGER,
        price REAL
    );`
    _, err := db.Exec(query)
    return err
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("OK"))
}

// 🛒 Add to cart or increase existing quantity
func addToCart(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "POST only", http.StatusMethodNotAllowed)
        return
    }

    var item CartItem
    if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    if item.ProductName == "" || item.Quantity <= 0 || item.Price <= 0 {
        http.Error(w, "Invalid data", http.StatusBadRequest)
        return
    }

    // Check if product already exists
    var existingQty int
    var existingPrice float64
    err := db.QueryRow("SELECT quantity, price FROM cart_items WHERE product_name = ?", item.ProductName).Scan(&existingQty, &existingPrice)
    if err == sql.ErrNoRows {
        // New product
        _, err = db.Exec("INSERT INTO cart_items (product_name, quantity, price) VALUES (?, ?, ?)",
            item.ProductName, item.Quantity, item.Price*float64(item.Quantity))
    } else if err == nil {
        // Update existing quantity
        newQty := existingQty + item.Quantity
        newPrice := item.Price * float64(newQty)
        _, err = db.Exec("UPDATE cart_items SET quantity=?, price=? WHERE product_name=?",
            newQty, newPrice, item.ProductName)
    }

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Write([]byte("Item added or updated successfully"))
}

// 🔄 Update cart quantity (increase or decrease)
func updateCart(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "POST only", http.StatusMethodNotAllowed)
        return
    }

    var update struct {
        ProductName string `json:"product_name"`
        Change      int    `json:"change"` // +1 to add, -1 to subtract
        UnitPrice   float64 `json:"unit_price"`
    }

    if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    var currentQty int
    err := db.QueryRow("SELECT quantity FROM cart_items WHERE product_name = ?", update.ProductName).Scan(&currentQty)
    if err == sql.ErrNoRows {
        http.Error(w, "Item not found in cart", http.StatusNotFound)
        return
    }

    newQty := currentQty + update.Change
    if newQty <= 0 {
        _, err = db.Exec("DELETE FROM cart_items WHERE product_name = ?", update.ProductName)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Write([]byte("Item removed from cart"))
        return
    }

    newPrice := update.UnitPrice * float64(newQty)
    _, err = db.Exec("UPDATE cart_items SET quantity=?, price=? WHERE product_name=?",
        newQty, newPrice, update.ProductName)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Write([]byte(fmt.Sprintf("Quantity updated to %d", newQty)))
}

func removeFromCart(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("product_name")
    if name == "" {
        http.Error(w, "Missing product_name parameter", http.StatusBadRequest)
        return
    }

    _, err := db.Exec("DELETE FROM cart_items WHERE product_name = ?", name)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Write([]byte("Item removed successfully"))
}

func viewCart(w http.ResponseWriter, r *http.Request) {
    rows, err := db.Query("SELECT id, product_name, quantity, price FROM cart_items")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var items []CartItem
    for rows.Next() {
        var item CartItem
        rows.Scan(&item.ID, &item.ProductName, &item.Quantity, &item.Price)
        items = append(items, item)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(items)
}

func checkout(w http.ResponseWriter, r *http.Request) {
    _, err := db.Exec("DELETE FROM cart_items")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Write([]byte("Checkout complete. Cart cleared."))
}

func withCORS(handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusOK)
            return
        }
        handler(w, r)
    }
}

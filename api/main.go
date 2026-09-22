// linkbin/api is a small URL-shortener backend: create a short code for a
// long URL, list what has been created, and redirect + count clicks.
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

var pg *sql.DB

const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newCode(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	out := make([]byte, n)
	for i, v := range b {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out)
}

func js(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func withCORSNoop(h http.HandlerFunc) http.HandlerFunc { return h } // same-origin via the Gate; no CORS needed

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	var err error
	pg, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if _, err := pg.Exec(`CREATE TABLE IF NOT EXISTS links (
		code TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		clicks INT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	http.HandleFunc("/api/health", withCORSNoop(func(w http.ResponseWriter, r *http.Request) {
		js(w, 200, map[string]any{"ok": true, "service": "linkbin-api"})
	}))

	http.HandleFunc("/api/links", withCORSNoop(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				URL string `json:"url"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
				js(w, 400, map[string]string{"error": "url is required"})
				return
			}
			u, err := url.Parse(body.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				js(w, 400, map[string]string{"error": "url must be a valid http(s) URL"})
				return
			}
			var code string
			for attempt := 0; attempt < 5; attempt++ {
				code = newCode(6)
				_, err = pg.Exec(`INSERT INTO links (code, url) VALUES ($1, $2)`, code, body.URL)
				if err == nil {
					break
				}
			}
			if err != nil {
				js(w, 500, map[string]string{"error": "could not create link"})
				return
			}
			js(w, 201, map[string]string{"code": code, "url": body.URL})
		case http.MethodGet:
			rows, err := pg.Query(`SELECT code, url, clicks, created_at FROM links ORDER BY created_at DESC LIMIT 100`)
			if err != nil {
				js(w, 500, map[string]string{"error": err.Error()})
				return
			}
			defer rows.Close()
			type link struct {
				Code      string `json:"code"`
				URL       string `json:"url"`
				CreatedAt string `json:"created_at"`
				Clicks    int    `json:"clicks"`
			}
			out := []link{}
			for rows.Next() {
				var l link
				rows.Scan(&l.Code, &l.URL, &l.Clicks, &l.CreatedAt)
				out = append(out, l)
			}
			js(w, 200, map[string]any{"links": out})
		default:
			js(w, 405, map[string]string{"error": "method not allowed"})
		}
	}))

	// GET /api/r/{code} — redirect to the original URL and count the click.
	http.HandleFunc("/api/r/", func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimPrefix(r.URL.Path, "/api/r/")
		var target string
		err := pg.QueryRow(`UPDATE links SET clicks = clicks + 1 WHERE code = $1 RETURNING url`, code).Scan(&target)
		if err == sql.ErrNoRows {
			js(w, 404, map[string]string{"error": "no such link"})
			return
		}
		if err != nil {
			js(w, 500, map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	})

	http.HandleFunc("/api/whoami", withCORSNoop(func(w http.ResponseWriter, r *http.Request) {
		js(w, 200, map[string]any{"gate_email": r.Header.Get("X-Gate-Email"), "gate_app": r.Header.Get("X-Gate-App")})
	}))

	log.Println("linkbin-api listening on 8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

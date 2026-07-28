// Command createadmin bootstraps a shop and its first owner account.
//
// Production databases are built from migrations/*.sql, which is schema only —
// the demo rows in migrations/seed/ are deliberately never applied there. Since
// there is no self-serve signup endpoint, a fresh deployment has no users and
// nobody can log in. This command creates that first account.
//
// Usage:
//
//	createadmin -shop "Bob's Garage" -email owner@example.com [-name "Bob"] [-password secret]
//
// With no -password, a strong one is generated and printed once. DATABASE_URL
// is read from the environment.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/garageflow/api-go/internal/db"
)

// defaultTemplate mirrors the "Courtesy Check" seeded for the demo shop, so a
// real shop starts with a usable DVI template instead of an empty list.
const defaultTemplate = `[
    {"section":"Brakes","label":"Front brake pads"},
    {"section":"Brakes","label":"Rear brake pads"},
    {"section":"Brakes","label":"Brake fluid"},
    {"section":"Tires","label":"Front tire tread"},
    {"section":"Tires","label":"Rear tire tread"},
    {"section":"Tires","label":"Tire pressure"},
    {"section":"Fluids","label":"Engine oil level"},
    {"section":"Fluids","label":"Coolant level"},
    {"section":"Fluids","label":"Washer fluid"},
    {"section":"Battery & Electrical","label":"Battery health"},
    {"section":"Battery & Electrical","label":"Headlights / taillights"},
    {"section":"Under Hood","label":"Serpentine belt"},
    {"section":"Under Hood","label":"Air filter"},
    {"section":"Wipers","label":"Wiper blades"}
]`

var (
	// Apostrophes are dropped rather than treated as separators, so
	// "Bob's Garage" slugs to "bobs-garage" and not "bob-s-garage".
	apostrophes = regexp.MustCompile(`['\x60\x{2019}]`)
	nonSlug     = regexp.MustCompile(`[^a-z0-9]+`)
)

func slugify(s string) string {
	s = apostrophes.ReplaceAllString(strings.ToLower(s), "")
	return strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
}

// generatePassword returns a URL-safe random password with 192 bits of entropy.
func generatePassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func main() {
	shopName := flag.String("shop", "", "shop name (required)")
	slug := flag.String("slug", "", "shop slug (defaults to a slugified shop name)")
	email := flag.String("email", "", "owner email address (required)")
	name := flag.String("name", "", "owner display name (defaults to the email local part)")
	password := flag.String("password", "", "owner password (generated and printed if omitted)")
	flag.Parse()

	if *shopName == "" || *email == "" {
		flag.Usage()
		log.Fatal("createadmin: -shop and -email are required")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("createadmin: DATABASE_URL is not set")
	}

	if *slug == "" {
		*slug = slugify(*shopName)
	}
	if *name == "" {
		*name, _, _ = strings.Cut(*email, "@")
	}

	generated := false
	if *password == "" {
		p, err := generatePassword()
		if err != nil {
			log.Fatalf("createadmin: generate password: %v", err)
		}
		*password, generated = p, true
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("createadmin: hash password: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("createadmin: connect: %v", err)
	}
	defer pool.Close()

	// One transaction: a half-created shop with no owner would leave the
	// deployment just as unusable as no shop at all.
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("createadmin: begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var existing string
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, *email).Scan(&existing)
	if err == nil {
		log.Fatalf("createadmin: a user with email %s already exists", *email)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("createadmin: check existing user: %v", err)
	}

	var shopID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO shops (name, slug) VALUES ($1, $2) RETURNING id`,
		*shopName, *slug,
	).Scan(&shopID); err != nil {
		log.Fatalf("createadmin: create shop: %v", err)
	}

	var userID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (shop_id, email, name, role, password_hash)
		 VALUES ($1, $2, $3, 'owner', $4) RETURNING id`,
		shopID, *email, *name, string(hash),
	).Scan(&userID); err != nil {
		log.Fatalf("createadmin: create owner: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO inspection_templates (shop_id, name, is_default, items)
		 VALUES ($1, 'Courtesy Check', true, $2)`,
		shopID, defaultTemplate,
	); err != nil {
		log.Fatalf("createadmin: create default inspection template: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("createadmin: commit: %v", err)
	}

	fmt.Printf("Created shop %q (%s)\n", *shopName, shopID)
	fmt.Printf("Created owner %s (%s)\n", *email, userID)
	fmt.Println("Seeded the default \"Courtesy Check\" inspection template.")
	if generated {
		fmt.Printf("\nGenerated password (shown once — store it now):\n\n    %s\n\n", *password)
	}
}

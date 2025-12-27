#!/bin/bash
# Seed super admin user for development

# Generate password hash for "admin123"
# Using Go to generate bcrypt hash
go run -C /Users/pande/Projects/ngrok-clone << 'EOF'
package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	// Open database
	db, err := sql.Open("sqlite3", "./grok.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Insert super admin user
	userID := uuid.New().String()
	_, err = db.Exec(`
		INSERT INTO users (id, email, password, name, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, userID, "admin", string(hash), "Super Admin", "super_admin", true)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✅ Super admin created successfully!")
	fmt.Println("   Email: admin")
	fmt.Println("   Password: admin123")
	fmt.Println("   Role: super_admin")
}
EOF

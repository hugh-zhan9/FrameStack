package database_test

import (
	"testing"

	"idea/internal/database"
)

func TestOpenRequiresDatabaseURL(t *testing.T) {
	db, err := database.Open("")
	if err == nil {
		t.Fatal("expected open to fail when database url is empty")
	}
	if db != nil {
		t.Fatal("expected db to be nil on empty database url")
	}
}

func TestOpenReturnsDatabaseHandleWithoutConnecting(t *testing.T) {
	db, err := database.Open("postgres://localhost:5432/idea?sslmode=disable")
	if err != nil {
		t.Fatalf("expected open to succeed: %v", err)
	}
	if db == nil {
		t.Fatal("expected db handle")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("expected db close to succeed: %v", err)
	}
}

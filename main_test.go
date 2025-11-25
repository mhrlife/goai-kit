package goaikit

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load()

	code := m.Run()
	os.Exit(code)
}

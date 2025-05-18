package goaikit

import (
	"github.com/joho/godotenv"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load()

	code := m.Run()
	os.Exit(code)
}

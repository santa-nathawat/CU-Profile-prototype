// Package config reads process settings from the environment.
package config

import "os"

type Config struct {
	Port         string
	DatabaseURL  string
	DemoSignIn   bool
	CookieSecure bool
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	return Config{
		Port:        port,
		DatabaseURL: os.Getenv("DATABASE_URL"),
		// Only the exact value "true" turns these on, so a typo can't open demo sign-in.
		DemoSignIn:   os.Getenv("DEMO_SIGN_IN") == "true",
		CookieSecure: os.Getenv("COOKIE_SECURE") == "true",
	}
}

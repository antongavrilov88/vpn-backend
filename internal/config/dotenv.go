package config

import (
	"os"
	"sync"

	"github.com/joho/godotenv"
)

var loadDotenvOnce sync.Once

func ensureDotenvLoaded() {
	loadDotenvOnce.Do(func() {
		_ = loadDotenvFromFiles(dotenvPaths())
	})
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dotenvPaths() []string {
	paths := []string{}

	if fileExists(".env") {
		paths = append(paths, ".env")
	}

	if fileExists(".env.local") {
		paths = append(paths, ".env.local")
	}

	return paths
}

func loadDotenvFromFiles(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	merged := make(map[string]string)
	for _, path := range paths {
		values, err := godotenv.Read(path)
		if err != nil {
			return err
		}

		for key, value := range values {
			merged[key] = value
		}
	}

	for key, value := range merged {
		current, present := os.LookupEnv(key)
		if present && current != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return nil
}

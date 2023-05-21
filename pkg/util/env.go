package util

import (
	"bufio"
	"os"
	"path/filepath"

	"github.com/fly-apps/postgres-ha/pkg/flypg"
)

func BuildEnv() ([]string, error) {
	node, err := flypg.NewNode()
	pathToEnv := filepath.Join(node.DataDir, ".env")

	file, err := os.Open(pathToEnv)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		env = append(env, scanner.Text())
	}

	return env, scanner.Err()
}

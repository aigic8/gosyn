package utils

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cespare/xxhash"
)

func IsSubPath(parent, sub string) (bool, error) {
	up := ".." + string(os.PathSeparator)

	// path-comparisons using filepath.Abs don't work reliably according to docs (no unique representation).
	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true, nil
	}
	return false, nil
}

func SplitEndpointAndFile(rawPath string) (string, string, error) {
	parts := strings.SplitN(rawPath, "/", 2)
	if len(parts) < 2 {
		return "", "", errors.New("endpoint is empty")
	}

	endpoint := strings.TrimSpace(parts[0])
	if endpoint == "" {
		return "", "", errors.New("endpoint is empty")
	}

	filePath := strings.TrimSpace(parts[1])
	if filePath == "" {
		return "", "", errors.New("file path is empty")
	}

	return endpoint, filePath, nil
}

func HashFile(filePath string) (string, error) {
	var hash string

	file, err := os.Open(filePath)
	if err != nil {
		return hash, err
	}
	defer file.Close()

	hashWriter := xxhash.New()
	if _, err := io.Copy(hashWriter, file); err != nil {
		return hash, err
	}
	hashBytes := hashWriter.Sum(nil)

	hash = hex.EncodeToString(hashBytes)
	return hash, nil
}

// TODO maybe use pointers for LastMod and Size? since they can be empty
type TreePath struct {
	Name     string              `json:"name"`
	IsDir    bool                `json:"isDir"`
	Size     int64               `json:"size"`
	LastMod  time.Time           `json:"lastModifiction"`
	Children map[string]TreePath `json:"children"`
}

func MakeTree(base string, tree map[string]TreePath) error {
	for key, item := range tree {
		if !item.IsDir {
			continue
		}

		curr := path.Join(base, item.Name)
		children, err := os.ReadDir(curr)
		if err != nil {
			return err
		}

		for _, child := range children {
			var size int64
			var modeTime time.Time
			if !child.IsDir() {
				info, err := child.Info()
				if err != nil {
					return fmt.Errorf("error getting fileinfo: %v", err)
				}
				size = info.Size()
				modeTime = info.ModTime()
			}
			tree[key].Children[child.Name()] = TreePath{
				IsDir:    child.IsDir(),
				Name:     child.Name(),
				Size:     size,
				LastMod:  modeTime,
				Children: map[string]TreePath{},
			}
		}

		if err = MakeTree(curr, tree[key].Children); err != nil {
			return err
		}
	}

	return nil
}

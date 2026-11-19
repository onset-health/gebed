// Export a read-only UI snapshot from one Beads repository without opening a network listener.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gebed/internal/registry"
	"gebed/internal/server"
	"gebed/internal/store"
	"gebed/internal/theme"
)

func export(dir string) error {
	temp, err := os.MkdirTemp("", "gebed-export-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	reg, err := registry.Load(filepath.Join(temp, "repos.json"))
	if err != nil {
		return err
	}
	if _, err = reg.AddNamed("forge", dir); err != nil {
		return err
	}
	handler := server.New(reg, store.Exec, theme.Builtin, nil, "")
	routes := map[string]json.RawMessage{}
	read := func(path string) error {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !json.Valid(rec.Body.Bytes()) {
			return fmt.Errorf("cannot export %s (HTTP %d)", path, rec.Code)
		}
		routes[path] = rec.Body.Bytes()
		return nil
	}
	const root = "/api/repos/forge"
	for _, p := range []string{"/api/themes", root + "/beads", root + "/edges", root + "/version", root + "/metrics"} {
		if err := read(p); err != nil {
			return err
		}
	}
	var beads []struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(routes[root+"/beads"], &beads); err != nil {
		return err
	}
	for _, b := range beads {
		path := root + "/beads/" + url.PathEscape(b.ID)
		for _, suffix := range []string{"", "/comments"} {
			if err := read(path + suffix); err != nil {
				return err
			}
		}
	}
	backend, err := store.Detect(dir)
	if err != nil {
		return err
	}
	routes["/api/repos"], err = json.Marshal([]map[string]string{{"name": "forge", "dir": "forge", "backend": backend.String()}})
	if err != nil {
		return err
	}
	routes["/api/sessions"] = json.RawMessage("[]")
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"observedAt": time.Now().UTC().Format(time.RFC3339), "routes": routes})
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: gebed-export REPOSITORY")
	}
	if err := export(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

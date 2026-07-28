package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	root := flag.String("root", "web", "directory containing exported web files")
	host := flag.String("host", "127.0.0.1", "host or interface to bind")
	port := flag.Int("port", 3010, "port to listen on")
	flag.Parse()

	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		log.Fatalf("resolve web root: %v", err)
	}
	if info, err := os.Stat(rootAbs); err != nil || !info.IsDir() {
		log.Fatalf("web root not found: %s", rootAbs)
	}

	bindHost := normalizeHost(*host)
	addr := net.JoinHostPort(bindHost, strconv.Itoa(*port))
	log.Printf("Static web server listening at http://%s", addr)
	if bindHost == "0.0.0.0" || bindHost == "::" {
		log.Printf("SECURITY WARNING: Web UI is listening on all network interfaces without product login or RBAC; use only on a trusted administration network")
	}
	log.Printf("Serving files from: %s", rootAbs)

	server := &http.Server{
		Addr:    addr,
		Handler: staticHandler(rootAbs),
	}
	log.Fatal(server.ListenAndServe())
}

func normalizeHost(host string) string {
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "":
		return "127.0.0.1"
	case "+", "*", "0.0.0.0":
		return "0.0.0.0"
	default:
		return strings.TrimSpace(host)
	}
}

func staticHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// The bundled WebView2 profile can outlive an application upgrade. Keep local
		// console assets revalidated so a new OpenAD package never runs old scripts.
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		requestPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if requestPath == "." || requestPath == "" {
			requestPath = "index.html"
		}

		candidate := filepath.Join(root, filepath.FromSlash(requestPath))
		if !isWithinRoot(root, candidate) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			indexCandidate := filepath.Join(candidate, "index.html")
			if _, indexErr := os.Stat(indexCandidate); indexErr == nil {
				candidate = indexCandidate
			} else if htmlCandidate := candidate + ".html"; isWithinRoot(root, htmlCandidate) {
				if _, htmlErr := os.Stat(htmlCandidate); htmlErr == nil {
					candidate = htmlCandidate
				} else {
					candidate = indexCandidate
				}
			} else {
				candidate = indexCandidate
			}
		}

		if _, err := os.Stat(candidate); err != nil {
			if htmlCandidate := candidate + ".html"; isWithinRoot(root, htmlCandidate) {
				if _, htmlErr := os.Stat(htmlCandidate); htmlErr == nil {
					candidate = htmlCandidate
				}
			}
		}

		if _, err := os.Stat(candidate); err != nil {
			if dynamicCandidate := dynamicRouteCandidate(root, requestPath); dynamicCandidate != "" {
				candidate = dynamicCandidate
			}
		}

		if _, err := os.Stat(candidate); err != nil {
			candidate = filepath.Join(root, "index.html")
		}

		http.ServeFile(w, r, candidate)
	})
}

func dynamicRouteCandidate(root, requestPath string) string {
	dir, file := filepath.Split(filepath.FromSlash(requestPath))
	if dir == "" || file == "" || strings.Contains(file, ".") {
		return ""
	}

	candidate := filepath.Join(root, dir, "[id].html")
	if !isWithinRoot(root, candidate) {
		return ""
	}
	if _, err := os.Stat(candidate); err != nil {
		return ""
	}
	return candidate
}

func isWithinRoot(root, target string) bool {
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

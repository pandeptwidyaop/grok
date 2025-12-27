package fileserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "fileserver-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test files
	files := map[string]string{
		"index.html":       "<html><body>Index</body></html>",
		"about.html":       "<html><body>About</body></html>",
		"404.html":         "<html><body>Not Found</body></html>",
		"files/doc.txt":    "Document content",
		"files/data.csv":   "col1,col2\nval1,val2",
		"nested/deep/file": "Deep file",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	return tmpDir
}

func TestNewServer(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Root:       tmpDir,
				EnableGzip: true,
			},
			wantErr: false,
		},
		{
			name: "non-existent directory",
			config: Config{
				Root: "/non/existent/path",
			},
			wantErr: true,
		},
		{
			name: "file instead of directory",
			config: Config{
				Root: filepath.Join(tmpDir, "index.html"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServer(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServeFile(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	server, err := NewServer(Config{
		Root:       tmpDir,
		EnableGzip: false,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := server.Handler()

	tests := []struct {
		name           string
		path           string
		wantStatus     int
		wantBodyPrefix string
	}{
		{
			name:           "serve index.html",
			path:           "/",
			wantStatus:     http.StatusOK,
			wantBodyPrefix: "<html><body>Index</body></html>",
		},
		{
			name:           "serve about.html",
			path:           "/about.html",
			wantStatus:     http.StatusOK,
			wantBodyPrefix: "<html><body>About</body></html>",
		},
		{
			name:           "serve file in subdirectory",
			path:           "/files/doc.txt",
			wantStatus:     http.StatusOK,
			wantBodyPrefix: "Document content",
		},
		{
			name:       "404 for non-existent file",
			path:       "/nonexistent.html",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "directory traversal attempt",
			path:       "/../../../etc/passwd",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantBodyPrefix != "" {
				body := w.Body.String()
				if !strings.HasPrefix(body, tt.wantBodyPrefix) {
					t.Errorf("Body = %q, want prefix %q", body, tt.wantBodyPrefix)
				}
			}
		})
	}
}

func TestDirectoryListing(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	// Create a directory without index.html
	noIndexDir := filepath.Join(tmpDir, "noindex")
	if err := os.Mkdir(noIndexDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(noIndexDir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(noIndexDir, "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	server, err := NewServer(Config{
		Root: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/noindex/", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "file1.txt") {
		t.Error("Directory listing should contain file1.txt")
	}
	if !strings.Contains(body, "file2.txt") {
		t.Error("Directory listing should contain file2.txt")
	}
	if !strings.Contains(body, "Index of /noindex/") {
		t.Error("Directory listing should contain path")
	}
}

func TestCustom404(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	server, err := NewServer(Config{
		Root:          tmpDir,
		Custom404Path: filepath.Join(tmpDir, "404.html"),
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Not Found") {
		t.Error("Should use custom 404 page")
	}
}

func TestBasicAuth(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	server, err := NewServer(Config{
		Root:          tmpDir,
		BasicAuthUser: "testuser",
		BasicAuthPass: "testpass",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		{
			name:       "valid credentials",
			username:   "testuser",
			password:   "testpass",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid password",
			username:   "testuser",
			password:   "wrongpass",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid username",
			username:   "wronguser",
			password:   "testpass",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "no credentials",
			username:   "",
			password:   "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.username != "" || tt.password != "" {
				req.SetBasicAuth(tt.username, tt.password)
			}
			w := httptest.NewRecorder()

			server.Handler().ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestGzipCompression(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	server, err := NewServer(Config{
		Root:       tmpDir,
		EnableGzip: true,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	tests := []struct {
		name             string
		acceptEncoding   string
		wantContentEnc   string
		wantCompressed   bool
	}{
		{
			name:           "with gzip support",
			acceptEncoding: "gzip, deflate",
			wantContentEnc: "gzip",
			wantCompressed: true,
		},
		{
			name:           "without gzip support",
			acceptEncoding: "",
			wantContentEnc: "",
			wantCompressed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/index.html", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			w := httptest.NewRecorder()

			server.Handler().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Logf("Headers: %v", w.Header())
				t.Logf("Location: %s", w.Header().Get("Location"))
				t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
			}

			contentEnc := w.Header().Get("Content-Encoding")
			if contentEnc != tt.wantContentEnc {
				t.Errorf("Content-Encoding = %q, want %q", contentEnc, tt.wantContentEnc)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{1073741824, "1.00 GB"},
		{1610612736, "1.50 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatSize(tt.size)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.size, got, tt.want)
			}
		})
	}
}

func TestFindAvailablePort(t *testing.T) {
	port, err := FindAvailablePort()
	if err != nil {
		t.Fatalf("FindAvailablePort() error = %v", err)
	}

	if port <= 0 || port > 65535 {
		t.Errorf("Invalid port number: %d", port)
	}

	// Verify the port is actually available by trying to listen on it
	// (This is a best-effort check since the port could be taken between calls)
	server, err := NewServer(Config{
		Root: os.TempDir(),
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(strings.Join([]string{"127.0.0.1:", string(rune(port))}, ""))
	}()

	// Give it a moment to start
	// Note: This is a basic test, in production you'd want more robust checks
	server.Close()
}

func TestRedirectToTrailingSlash(t *testing.T) {
	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	server, err := NewServer(Config{
		Root: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/files", nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}

	location := w.Header().Get("Location")
	if location != "/files/" {
		t.Errorf("Location = %q, want %q", location, "/files/")
	}
}

func BenchmarkServeFile(b *testing.B) {
	tmpDir := setupTestDir(&testing.T{})
	defer os.RemoveAll(tmpDir)

	server, _ := NewServer(Config{
		Root: tmpDir,
	})

	handler := server.Handler()
	req := httptest.NewRequest("GET", "/index.html", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		_, _ = io.ReadAll(w.Body)
	}
}

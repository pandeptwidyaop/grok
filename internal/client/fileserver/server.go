package fileserver

import (
	"compress/gzip"
	"crypto/subtle"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// Config holds configuration for the file server.
type Config struct {
	// Root directory to serve files from
	Root string
	// EnableGzip enables gzip compression
	EnableGzip bool
	// Custom404Path path to custom 404.html file (optional)
	Custom404Path string
	// BasicAuth credentials (username:password), empty to disable
	BasicAuthUser string
	BasicAuthPass string
}

// Server is a static file server with directory listing.
type Server struct {
	cfg    Config
	server *http.Server
}

// NewServer creates a new file server instance.
func NewServer(cfg Config) (*Server, error) {
	// Validate root directory
	absRoot, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("cannot access root directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", absRoot)
	}

	cfg.Root = absRoot

	// Check for custom 404 page
	if cfg.Custom404Path == "" {
		custom404 := filepath.Join(absRoot, "404.html")
		if _, err := os.Stat(custom404); err == nil {
			cfg.Custom404Path = custom404
		}
	}

	return &Server{
		cfg: cfg,
	}, nil
}

// Handler returns the HTTP handler for the file server.
func (s *Server) Handler() http.Handler {
	handler := http.HandlerFunc(s.serveFile)

	// Apply middleware in reverse order (last applied = first executed)
	if s.cfg.BasicAuthUser != "" && s.cfg.BasicAuthPass != "" {
		handler = s.basicAuthMiddleware(handler)
	}

	if s.cfg.EnableGzip {
		handler = s.gzipMiddleware(handler)
	}

	return handler
}

// serveFile serves files and directory listings.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	// Clean the URL path to prevent directory traversal
	// Note: path.Clean removes trailing slashes, so we need to remember if it had one
	urlPath := path.Clean("/" + r.URL.Path)
	hasTrailingSlash := strings.HasSuffix(r.URL.Path, "/")

	// Restore trailing slash for directories if it was present
	if hasTrailingSlash && urlPath != "/" {
		urlPath += "/"
	}

	// Construct file path (strip leading / from urlPath to avoid absolute path issues)
	cleanPath := strings.TrimPrefix(strings.TrimSuffix(urlPath, "/"), "/")
	filePath := filepath.Join(s.cfg.Root, filepath.FromSlash(cleanPath))

	// Ensure the file path is within the root directory (prevent directory traversal)
	if !strings.HasPrefix(filePath, s.cfg.Root) {
		logger.WarnEvent().
			Str("requested_path", urlPath).
			Str("file_path", filePath).
			Msg("Directory traversal attempt detected")
		s.serve404(w, r)
		return
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.serve404(w, r)
			return
		}
		logger.ErrorEvent().
			Err(err).
			Str("path", filePath).
			Msg("Failed to stat file")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// If it's a directory
	if info.IsDir() {
		s.serveDirectory(w, r, filePath, urlPath)
		return
	}

	// Serve the file using ServeContent to avoid unwanted redirects
	file, err := os.Open(filePath)
	if err != nil {
		logger.ErrorEvent().Err(err).Str("path", filePath).Msg("Failed to open file")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	http.ServeContent(w, r, filepath.Base(filePath), info.ModTime(), file)
}

// serveDirectory serves directory listings or index.html.
func (s *Server) serveDirectory(w http.ResponseWriter, r *http.Request, dirPath, urlPath string) {
	// Ensure URL ends with /
	if !strings.HasSuffix(urlPath, "/") {
		http.Redirect(w, r, urlPath+"/", http.StatusMovedPermanently)
		return
	}

	// Try to serve index.html
	indexPath := filepath.Join(dirPath, "index.html")
	if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
		file, err := os.Open(indexPath)
		if err == nil {
			defer file.Close()
			http.ServeContent(w, r, "index.html", info.ModTime(), file)
			return
		}
	}

	// Serve directory listing
	s.serveDirectoryListing(w, r, dirPath, urlPath)
}

// serveDirectoryListing generates and serves an HTML directory listing.
func (s *Server) serveDirectoryListing(w http.ResponseWriter, r *http.Request, dirPath, urlPath string) {
	// Read directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("path", dirPath).
			Msg("Failed to read directory")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build file list
	type FileInfo struct {
		Name         string
		IsDir        bool
		Size         string
		ModTime      string
		ModTimeValue time.Time
		URL          string
	}

	var files []FileInfo
	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileInfo := FileInfo{
			Name:         entry.Name(),
			IsDir:        entry.IsDir(),
			ModTimeValue: info.ModTime(),
			ModTime:      info.ModTime().Format("2006-01-02 15:04:05"),
			URL:          path.Join(urlPath, entry.Name()),
		}

		if entry.IsDir() {
			fileInfo.Size = "-"
			fileInfo.URL += "/"
		} else {
			fileInfo.Size = formatSize(info.Size())
		}

		files = append(files, fileInfo)
	}

	// Sort: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	// Add parent directory link if not root
	if urlPath != "/" {
		parentURL := path.Dir(urlPath)
		if parentURL != urlPath {
			files = append([]FileInfo{{
				Name:  "..",
				IsDir: true,
				Size:  "-",
				URL:   parentURL + "/",
			}}, files...)
		}
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := directoryListingTemplate.Execute(w, map[string]interface{}{
		"Path":  urlPath,
		"Files": files,
	}); err != nil {
		logger.ErrorEvent().
			Err(err).
			Msg("Failed to render directory listing")
	}
}

// serve404 serves a 404 error page.
func (s *Server) serve404(w http.ResponseWriter, r *http.Request) {
	// Try custom 404 page
	if s.cfg.Custom404Path != "" {
		if content, err := os.ReadFile(s.cfg.Custom404Path); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write(content)
			return
		}
	}

	// Default 404 page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>404 Not Found</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 100px auto; text-align: center; }
        h1 { color: #e74c3c; }
    </style>
</head>
<body>
    <h1>404 Not Found</h1>
    <p>The requested URL <code>%s</code> was not found on this server.</p>
</body>
</html>`, r.URL.Path)
}

// formatSize formats file size in human-readable format.
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// directoryListingTemplate is the HTML template for directory listings.
var directoryListingTemplate = template.Must(template.New("listing").Parse(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Index of {{.Path}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        h1 {
            background: #2c3e50;
            color: white;
            padding: 20px;
            font-size: 20px;
            font-weight: 500;
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th {
            background: #ecf0f1;
            padding: 12px 20px;
            text-align: left;
            font-weight: 600;
            color: #2c3e50;
            border-bottom: 2px solid #bdc3c7;
        }
        td {
            padding: 12px 20px;
            border-bottom: 1px solid #ecf0f1;
        }
        tr:hover {
            background: #f8f9fa;
        }
        a {
            color: #3498db;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        .dir { color: #f39c12; font-weight: 500; }
        .file { color: #2c3e50; }
        .size { color: #7f8c8d; text-align: right; }
        .date { color: #95a5a6; }
        .footer {
            padding: 15px 20px;
            background: #ecf0f1;
            text-align: center;
            color: #7f8c8d;
            font-size: 14px;
        }
        @media (max-width: 768px) {
            .date { display: none; }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Index of {{.Path}}</h1>
        <table>
            <thead>
                <tr>
                    <th>Name</th>
                    <th style="width: 150px; text-align: right;">Size</th>
                    <th style="width: 200px;" class="date">Modified</th>
                </tr>
            </thead>
            <tbody>
                {{range .Files}}
                <tr>
                    <td>
                        <a href="{{.URL}}" class="{{if .IsDir}}dir{{else}}file{{end}}">
                            {{if .IsDir}}📁{{else}}📄{{end}} {{.Name}}{{if .IsDir}}/{{end}}
                        </a>
                    </td>
                    <td class="size">{{.Size}}</td>
                    <td class="date">{{.ModTime}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        <div class="footer">Powered by Grok File Server</div>
    </div>
</body>
</html>
`))

// gzipMiddleware compresses responses with gzip.
func (s *Server) gzipMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Create gzip writer
		gz := gzip.NewWriter(w)
		defer gz.Close()

		// Wrap response writer
		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			Writer:         gz,
		}

		w.Header().Set("Content-Encoding", "gzip")
		next.ServeHTTP(gzw, r)
	}
}

// gzipResponseWriter wraps http.ResponseWriter to write gzipped responses.
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// basicAuthMiddleware adds HTTP Basic Authentication.
func (s *Server) basicAuthMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()

		// Use constant-time comparison to prevent timing attacks
		usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(s.cfg.BasicAuthUser)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.BasicAuthPass)) == 1

		if !ok || !usernameMatch || !passwordMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="Grok File Server"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized\n"))
			return
		}

		next.ServeHTTP(w, r)
	}
}

// Start starts the file server on the given address.
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.InfoEvent().
		Str("addr", addr).
		Str("root", s.cfg.Root).
		Msg("Starting file server")

	return s.server.ListenAndServe()
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var templates *template.Template

func initTemplates() error {
	tmpl := template.New("")
	
	// Add custom template functions
	tmpl.Funcs(template.FuncMap{
		"formatBytes": formatBytes,
		"formatDuration": formatDuration,
	})

	// Parse all templates - now using unique content template names
	// Each page defines its own "content" that calls a unique template
	// This avoids the "last parsed wins" issue
	templateFiles := []string{
		"web/templates/base.html",
		"web/templates/media_list.html",
		"web/templates/login.html",
		"web/templates/dashboard.html",
		"web/templates/setup.html",
		"web/templates/admin.html",
		"web/templates/settings.html",
	}
	
	for _, file := range templateFiles {
		if _, err := os.Stat(file); err == nil {
			_, err := tmpl.ParseFiles(file)
			if err != nil {
				slog.Error("Failed to parse template", "file", file, "error", err)
				return err
			}
			relPath, _ := filepath.Rel("web/templates", file)
			slog.Info("Loaded template", "file", relPath)
		}
	}

	templates = tmpl
	return nil
}

func (s *Server) renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) error {
	if templates == nil {
		return fmt.Errorf("templates not initialized")
	}
	
	// The problem: Go templates use the LAST parsed definition when multiple templates
	// define the same block name. Since dashboard.html is parsed last, its "content" always wins.
	//
	// Solution: Use template.Clone() to create separate template sets for each page,
	// OR dynamically re-parse templates in the right order.
	//
	// Simpler solution: Create a wrapper template that includes base.html + the specific content.
	// But Go templates don't work that way easily.
	//
	// Best solution: Each page template is self-contained (includes base structure).
	// But that's duplication. Let's use a different approach:
	//
	// Parse templates dynamically based on which page we're rendering.
	// We'll create a new template set for each request, parsing in the right order.
	
	// For now, let's try cloning and re-parsing just the needed template
	// Actually, simpler: Parse login.html AFTER dashboard.html when rendering login
	// We can do this by re-parsing just that template into a clone
	
	// Fix: Create a fresh template set for each page with the target template parsed last
	// This ensures the correct "content" definition is used
	tmplInstance := template.New("")
	tmplInstance.Funcs(template.FuncMap{
		"formatBytes":    formatBytes,
		"formatDuration": formatDuration,
	})
	
	// Determine which template should be parsed last
	allTemplates := []string{
		"web/templates/base.html",
		"web/templates/media_list.html",
		"web/templates/login.html",
		"web/templates/dashboard.html",
		"web/templates/setup.html",
		"web/templates/admin.html",
		"web/templates/settings.html",
	}
	
	// Reorder templates to put the target template last
	templateFiles := []string{}
	for _, file := range allTemplates {
		if !strings.HasSuffix(file, tmpl) {
			templateFiles = append(templateFiles, file)
		}
	}
	// Add the target template last
	templateFiles = append(templateFiles, "web/templates/"+tmpl)
	
	for _, file := range templateFiles {
		if _, err := os.Stat(file); err == nil {
			_, err := tmplInstance.ParseFiles(file)
			if err != nil {
				return fmt.Errorf("failed to parse template %s: %w", file, err)
			}
		}
	}
	
	return tmplInstance.ExecuteTemplate(w, "base.html", data)
}

// Helper functions for templates
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	return fmt.Sprintf("%dd", seconds/86400)
}


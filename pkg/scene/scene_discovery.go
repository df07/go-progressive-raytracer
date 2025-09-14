package scene

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SceneInfo represents a discovered scene with its metadata
type SceneInfo struct {
	ID          string `json:"id"`          // Unique identifier
	Name        string `json:"name"`        // Scene name
	DisplayName string `json:"displayName"` // UI display name
	Description string `json:"description"` // Optional description
	Group       string `json:"group"`       // Grouping category
	Type        string `json:"type"`        // "builtin" or "pbrt"
	FilePath    string `json:"filePath"`    // Path to PBRT file (pbrt type only)
	Variant     string `json:"variant"`     // Variant name (optional)
}

// SceneGroup represents a group of related scenes
type SceneGroup struct {
	Name   string      `json:"name"`
	Scenes []SceneInfo `json:"scenes"`
}

// ScenesResponse represents the complete response for /api/scenes
type ScenesResponse struct {
	Groups []SceneGroup `json:"groups"`
}

// ListPBRTScenes scans the /scenes directory and returns discovered PBRT scenes
func ListPBRTScenes() ([]SceneInfo, error) {
	// Try different possible paths for scenes directory
	possiblePaths := []string{"scenes", "../scenes"}
	var scenesDir string

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			scenesDir = path
			break
		}
	}

	if scenesDir == "" {
		// No scenes directory found, return empty list
		return []SceneInfo{}, nil
	}

	// Find all .pbrt files in the scenes directory
	pattern := filepath.Join(scenesDir, "*.pbrt")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan scenes directory: %v", err)
	}

	var scenes []SceneInfo
	for _, filePath := range files {
		sceneInfo, err := ParsePBRTMetadata(filePath)
		if err != nil {
			// Log warning but continue processing other files
			fmt.Printf("Warning: failed to parse metadata for %s: %v\n", filePath, err)
			continue
		}
		scenes = append(scenes, sceneInfo)
	}

	// Sort scenes by display name
	sort.Slice(scenes, func(i, j int) bool {
		return scenes[i].DisplayName < scenes[j].DisplayName
	})

	return scenes, nil
}

// ParsePBRTMetadata extracts metadata from PBRT file header comments
func ParsePBRTMetadata(filePath string) (SceneInfo, error) {
	// Extract filename without extension for fallback values
	filename := filepath.Base(filePath)
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Create SceneInfo with fallback values
	sceneInfo := SceneInfo{
		ID:          fmt.Sprintf("pbrt:%s", nameWithoutExt),
		Name:        titleCase(nameWithoutExt),
		DisplayName: titleCase(nameWithoutExt),
		Description: "",
		Group:       "PBRT Scenes", // Default group
		Type:        "pbrt",
		FilePath:    filePath,
		Variant:     "",
	}

	// Open file to read header comments
	file, err := os.Open(filePath)
	if err != nil {
		// If we can't read the file, return with fallback values
		return sceneInfo, nil
	}
	defer file.Close()

	// Read header comments to extract metadata
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Stop parsing at first non-comment line
		if !strings.HasPrefix(line, "#") {
			break
		}

		// Parse metadata from comment
		if strings.HasPrefix(line, "# ") {
			content := strings.TrimPrefix(line, "# ")

			if strings.HasPrefix(content, "Scene:") {
				sceneInfo.Name = strings.TrimSpace(strings.TrimPrefix(content, "Scene:"))
			} else if strings.HasPrefix(content, "Variant:") {
				sceneInfo.Variant = strings.TrimSpace(strings.TrimPrefix(content, "Variant:"))
			} else if strings.HasPrefix(content, "Description:") {
				sceneInfo.Description = strings.TrimSpace(strings.TrimPrefix(content, "Description:"))
			} else if strings.HasPrefix(content, "Group:") {
				sceneInfo.Group = strings.TrimSpace(strings.TrimPrefix(content, "Group:"))
			}
		}
	}

	// Update display name based on parsed metadata
	if sceneInfo.Variant != "" {
		sceneInfo.DisplayName = fmt.Sprintf("%s - %s", sceneInfo.Name, sceneInfo.Variant)
	} else {
		sceneInfo.DisplayName = sceneInfo.Name
	}

	return sceneInfo, scanner.Err()
}

// ListAllScenes returns both built-in and PBRT scenes, grouped by category
func ListAllScenes() (ScenesResponse, error) {
	var response ScenesResponse

	// Add built-in scenes (hardcoded for now - Phase 1)
	builtInScenes := []SceneInfo{
		{
			ID:          "cornell-box",
			Name:        "Cornell Box",
			DisplayName: "Cornell Box",
			Description: "Cornell box with two spheres",
			Group:       "Built-in Scenes",
			Type:        "builtin",
		},
		{
			ID:          "basic",
			Name:        "Default Scene",
			DisplayName: "Default Scene",
			Description: "Basic scene with spheres and plane ground",
			Group:       "Built-in Scenes",
			Type:        "builtin",
		},
		{
			ID:          "sphere-grid",
			Name:        "Sphere Grid",
			DisplayName: "Sphere Grid",
			Description: "10x10 grid of rainbow-colored metallic spheres",
			Group:       "Built-in Scenes",
			Type:        "builtin",
		},
		{
			ID:          "triangle-mesh-sphere",
			Name:        "Triangle Mesh Sphere",
			DisplayName: "Triangle Mesh Sphere",
			Description: "Scene showcasing triangle mesh geometry",
			Group:       "Built-in Scenes",
			Type:        "builtin",
		},
		{
			ID:          "dragon",
			Name:        "Dragon PLY Mesh",
			DisplayName: "Dragon PLY Mesh",
			Description: "Dragon PLY mesh from PBRT book",
			Group:       "Built-in Scenes",
			Type:        "builtin",
		},
		{
			ID:          "caustic-glass",
			Name:        "Caustic Glass",
			DisplayName: "Caustic Glass",
			Description: "Glass scene with complex geometry for testing caustics",
			Group:       "Built-in Scenes",
			Type:        "builtin",
		},
	}

	// Get PBRT scenes
	pbrtScenes, err := ListPBRTScenes()
	if err != nil {
		return response, fmt.Errorf("failed to list PBRT scenes: %v", err)
	}

	// Combine all scenes
	allScenes := append(builtInScenes, pbrtScenes...)

	// Group scenes by their Group field
	groupMap := make(map[string][]SceneInfo)
	for _, scene := range allScenes {
		groupMap[scene.Group] = append(groupMap[scene.Group], scene)
	}

	// Create ordered groups (Built-in first, then alphabetical)
	var groupNames []string
	for groupName := range groupMap {
		if groupName != "Built-in Scenes" {
			groupNames = append(groupNames, groupName)
		}
	}
	sort.Strings(groupNames)

	// Add built-in scenes group first if it exists
	if builtInGroup, exists := groupMap["Built-in Scenes"]; exists {
		response.Groups = append(response.Groups, SceneGroup{
			Name:   "Built-in Scenes",
			Scenes: builtInGroup,
		})
	}

	// Add other groups alphabetically
	for _, groupName := range groupNames {
		response.Groups = append(response.Groups, SceneGroup{
			Name:   groupName,
			Scenes: groupMap[groupName],
		})
	}

	return response, nil
}

// titleCase converts a filename-style string to title case
// e.g., "cornell-empty" -> "Cornell Empty"
func titleCase(s string) string {
	// Replace hyphens and underscores with spaces
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")

	// Title case each word
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

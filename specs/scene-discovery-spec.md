# Scene Discovery Specification

## Overview

The Scene Discovery system enables automatic detection and loading of PBRT scene files from the `/scenes` directory, allowing users to add new scenes without modifying source code. This document specifies the design, metadata format, and implementation approach.

## Goals

1. **Zero-config scene addition**: Drop `.pbrt` files in `/scenes` and they appear in the web interface
2. **Metadata support**: Allow scenes to specify display names, descriptions, and grouping
3. **Backward compatibility**: Existing built-in scenes continue to work unchanged
4. **CLI flexibility**: Support both built-in shortcuts and direct file paths
5. **Incremental migration**: Gradually move built-in scenes to PBRT format

## PBRT File Metadata Format

### Header Comment Syntax

PBRT scenes can include metadata in header comments using this format:

```pbrt
# Scene: Cornell Box
# Variant: Empty Room
# Description: Classic Cornell box with no objects for testing area lighting
# Group: Cornell Variants

LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40
# ... rest of scene ...
```

### Metadata Fields

| Field | Required | Description | Example |
|-------|----------|-------------|---------|
| `Scene` | No | Base scene name for grouping | `Cornell Box` |
| `Variant` | No | Specific variant name | `Empty Room` |
| `Description` | No | Scene description for tooltips | `Classic Cornell box...` |
| `Group` | No | Category for organization | `Cornell Variants` |

### Fallback Behavior

For PBRT files without metadata headers:
- **Scene name**: Derived from filename (`cornell-empty.pbrt` → `Cornell Empty`)
- **Display name**: Titlecase conversion of filename
- **Description**: Empty string
- **Group**: `"PBRT Scenes"` (default group)

## Scene Discovery API

### Data Structures

```go
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

type SceneGroup struct {
    Name   string      `json:"name"`
    Scenes []SceneInfo `json:"scenes"`
}

type ScenesResponse struct {
    Groups []SceneGroup `json:"groups"`
}
```

### Core Functions

```go
// ListPBRTScenes scans /scenes directory and returns discovered PBRT scenes
func ListPBRTScenes() ([]SceneInfo, error)

// ParsePBRTMetadata extracts metadata from PBRT file header comments
func ParsePBRTMetadata(filePath string) (SceneInfo, error)

// ListAllScenes returns both built-in and PBRT scenes, grouped
func ListAllScenes() (ScenesResponse, error)
```

## Web API Specification

### GET /api/scenes

Returns all available scenes grouped by category.

**Response Format:**
```json
{
  "groups": [
    {
      "name": "Built-in Scenes",
      "scenes": [
        {
          "id": "cornell-box",
          "name": "Cornell Box",
          "displayName": "Cornell Box",
          "description": "Cornell box with two spheres",
          "group": "Built-in Scenes",
          "type": "builtin"
        }
      ]
    },
    {
      "name": "Cornell Variants",
      "scenes": [
        {
          "id": "pbrt:cornell-empty",
          "name": "Cornell Box",
          "displayName": "Cornell Box - Empty Room",
          "description": "Classic Cornell box with no objects",
          "group": "Cornell Variants",
          "type": "pbrt",
          "filePath": "scenes/cornell-empty.pbrt",
          "variant": "Empty Room"
        }
      ]
    }
  ]
}
```

## CLI Interface

### Current Behavior (Preserved)
```bash
./raytracer --scene=cornell-box    # Built-in Cornell box
./raytracer --scene=default        # Built-in default scene
```

### New PBRT File Support (Future Phase)
```bash
./raytracer --scene=scenes/cornell-empty.pbrt    # Direct PBRT file
./raytracer --scene=pbrt:cornell-empty           # PBRT scene by ID
```

## Web Interface Specification

### Scene Selection Dropdown

The scene dropdown should be populated dynamically and organized by groups:

```html
<select id="scene">
  <optgroup label="Built-in Scenes">
    <option value="cornell-box">Cornell Box</option>
    <option value="default">Default Scene</option>
  </optgroup>
  <optgroup label="Cornell Variants">
    <option value="pbrt:cornell-empty">Cornell Box - Empty Room</option>
    <option value="pbrt:cornell-spheres">Cornell Box - Two Spheres</option>
  </optgroup>
  <optgroup label="PBRT Scenes">
    <option value="pbrt:my-custom-scene">My Custom Scene</option>
  </optgroup>
</select>
```

### Scene ID Format

- **Built-in scenes**: Use existing scene names (`"cornell-box"`, `"default"`)
- **PBRT scenes**: Use `"pbrt:"` prefix + filename without extension (`"pbrt:cornell-empty"`)

## Implementation Phases

### Phase 1: Foundation (Current Phase)
- [x] PBRT file format specification
- [ ] Scene discovery service implementation
- [ ] Web API endpoint (`/api/scenes`)
- [ ] Frontend integration (append to existing dropdown)
- [ ] Testing with existing PBRT files

### Phase 2: CLI Enhancement
- [ ] Modify CLI to accept PBRT file paths
- [ ] Maintain backward compatibility with built-in scenes
- [ ] Update help text and documentation

### Phase 3: Migration
- [ ] Create PBRT files for built-in scene variants
- [ ] Gradually remove hardcoded scene logic
- [ ] Fully dynamic scene system

## File Organization

```
/scenes/
├── cornell-empty.pbrt      # Scene: Cornell Box, Variant: Empty Room
├── cornell-spheres.pbrt    # Scene: Cornell Box, Variant: Two Spheres
├── cornell-boxes.pbrt      # Scene: Cornell Box, Variant: Rotated Boxes
├── dragon-gold.pbrt        # Scene: Dragon, Variant: Gold Material
├── caustic-glass.pbrt      # Scene: Caustic Glass
└── custom-scene.pbrt       # Scene: Custom Scene (no metadata)

/specs/
├── scene-discovery-spec.md # This specification
└── pbrt-format-spec.md     # PBRT format reference

/pkg/scene/
├── scene_discovery.go      # New scene discovery service
├── pbrt_scene.go          # Existing PBRT loader
└── cornell.go             # Existing built-in scenes
```

## Error Handling

- **Invalid PBRT files**: Log warning, skip file, continue scanning
- **Missing scenes directory**: Return empty PBRT scenes list
- **Permission errors**: Log error, return partial results
- **Malformed metadata**: Use fallback values, log warning
- **Web API errors**: Return 500 with error message

## Security Considerations

- **Path validation**: Only scan files within `/scenes` directory
- **File type restriction**: Only process `.pbrt` files
- **Input sanitization**: Sanitize metadata strings for XSS prevention
- **Resource limits**: Limit number of files scanned and metadata size

## Testing Strategy

### Unit Tests
- `TestListPBRTScenes()` - Directory scanning and metadata parsing
- `TestParsePBRTMetadata()` - Header comment parsing edge cases
- `TestListAllScenes()` - Integration of built-in and PBRT scenes

### Integration Tests
- End-to-end web API testing
- Frontend scene loading and selection
- CLI scene loading with PBRT files

### Test Scenarios
- PBRT files with complete metadata
- PBRT files with partial metadata
- PBRT files with no metadata
- Invalid PBRT files
- Empty scenes directory
- Permission denied scenarios

## Future Enhancements

- **Scene parameters**: Support for scene variants with configurable parameters
- **Scene previews**: Thumbnail generation for scene selection
- **Scene validation**: Validate PBRT syntax before listing
- **Hot reload**: Watch `/scenes` directory for changes
- **Scene categories**: More sophisticated grouping and tagging
- **Scene search**: Search and filter scenes by name/description
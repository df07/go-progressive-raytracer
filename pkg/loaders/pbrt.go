package loaders

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// PBRTStatement represents a parsed PBRT statement
type PBRTStatement struct {
	Type          string               // Statement type (Camera, Material, Shape, etc.)
	Subtype       string               // Subtype (perspective, diffuse, sphere, etc.)
	Parameters    map[string]PBRTParam // Named parameters
	MaterialIndex int                  // For shapes: index of material to use (-1 = no material)
}

// PBRTParam represents a parameter with type and value(s)
type PBRTParam struct {
	Type   string   // Parameter type (float, rgb, point3, etc.)
	Values []string // Parameter values as strings
}

// PBRTScene contains all parsed PBRT scene data
type PBRTScene struct {
	// Pre-WorldBegin statements
	Camera     *PBRTStatement
	LookAt     *core.Vec3 // Eye position
	LookAtTo   *core.Vec3 // Look at target
	LookAtUp   *core.Vec3 // Up vector
	Film       *PBRTStatement
	Sampler    *PBRTStatement
	Integrator *PBRTStatement

	// World content (inside WorldBegin/WorldEnd)
	Materials    []PBRTStatement
	Shapes       []PBRTStatement
	LightSources []PBRTStatement
	Transforms   []PBRTStatement
	Attributes   []AttributeBlock
}

// AttributeBlock represents an AttributeBegin/AttributeEnd block
type AttributeBlock struct {
	Materials    []PBRTStatement
	Shapes       []PBRTStatement
	LightSources []PBRTStatement
	Transforms   []PBRTStatement
}

// GraphicsState represents the current graphics state (for AttributeBegin/AttributeEnd stack)
type GraphicsState struct {
	MaterialIndex   int            // Current material index
	AreaLightSource *PBRTStatement // Current area light source (nil if none)
}

// PBRTParser encapsulates the state and logic for parsing PBRT files
type PBRTParser struct {
	scene                *PBRTScene
	attributeStack       []*AttributeBlock
	stateStack           []GraphicsState
	currentMaterialIndex int
	inWorld              bool
	statementLines       []string
}

// ParsePBRT parses PBRT content from an io.Reader
func ParsePBRT(reader io.Reader) (*PBRTScene, error) {
	// Create parser instance
	parser := NewPBRTParser()

	// Process each line
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if err := parser.processLine(scanner.Text()); err != nil {
			return nil, err
		}
	}

	// Process any remaining accumulated statements
	if err := parser.finalize(); err != nil {
		return nil, err
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input: %v", err)
	}

	return parser.scene, nil
}

// LoadPBRT loads and parses a PBRT scene file
func LoadPBRT(filename string) (*PBRTScene, error) {
	// Validate file path for security
	if err := validateFilePath(filename); err != nil {
		return nil, err
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PBRT file: %v", err)
	}
	defer file.Close()

	return ParsePBRT(file)
}

// NewPBRTParser creates a new PBRT parser instance
func NewPBRTParser() *PBRTParser {
	return &PBRTParser{
		scene: &PBRTScene{
			Materials:    make([]PBRTStatement, 0),
			Shapes:       make([]PBRTStatement, 0),
			LightSources: make([]PBRTStatement, 0),
			Transforms:   make([]PBRTStatement, 0),
			Attributes:   make([]AttributeBlock, 0),
		},
		attributeStack:       make([]*AttributeBlock, 0),
		stateStack:           make([]GraphicsState, 0),
		currentMaterialIndex: -1,
		inWorld:              false,
		statementLines:       make([]string, 0),
	}
}

// getCurrentAttribute returns the current attribute block from the stack, or nil if none
func (p *PBRTParser) getCurrentAttribute() *AttributeBlock {
	if len(p.attributeStack) > 0 {
		return p.attributeStack[len(p.attributeStack)-1]
	}
	return nil
}

// processAccumulatedStatement processes any accumulated statement lines and clears them
func (p *PBRTParser) processAccumulatedStatement(context string) error {
	if len(p.statementLines) > 0 {
		fullStatement := strings.Join(p.statementLines, " ")
		stmt, err := parseStatement(fullStatement)
		if err != nil {
			return fmt.Errorf("error parsing statement %s '%s': %v", context, fullStatement, err)
		}
		if err := p.routeStatement(stmt); err != nil {
			return err
		}
		p.statementLines = nil
	}
	return nil
}

// processWorldBegin handles WorldBegin directive
func (p *PBRTParser) processWorldBegin() error {
	if err := p.processAccumulatedStatement("before WorldBegin"); err != nil {
		return err
	}
	p.inWorld = true
	return nil
}

// processWorldEnd handles WorldEnd directive
func (p *PBRTParser) processWorldEnd() error {
	if err := p.processAccumulatedStatement("before WorldEnd"); err != nil {
		return err
	}
	p.inWorld = false
	return nil
}

// processAttributeBegin handles AttributeBegin directive
func (p *PBRTParser) processAttributeBegin() error {
	if err := p.processAccumulatedStatement("before AttributeBegin"); err != nil {
		return err
	}

	// Copy current state and push onto stack
	currentState := GraphicsState{
		MaterialIndex:   p.currentMaterialIndex,
		AreaLightSource: nil, // Area light state is inherited but starts fresh in new block
	}

	// Inherit area light state from parent if we're in nested attribute blocks
	if len(p.stateStack) > 0 {
		parentState := p.stateStack[len(p.stateStack)-1]
		currentState.AreaLightSource = parentState.AreaLightSource
	}

	p.stateStack = append(p.stateStack, currentState)

	// Create new attribute block and push onto stack
	newAttribute := &AttributeBlock{
		Materials:    make([]PBRTStatement, 0),
		Shapes:       make([]PBRTStatement, 0),
		LightSources: make([]PBRTStatement, 0),
		Transforms:   make([]PBRTStatement, 0),
	}
	p.attributeStack = append(p.attributeStack, newAttribute)
	return nil
}

// processAttributeEnd handles AttributeEnd directive
func (p *PBRTParser) processAttributeEnd() error {
	if err := p.processAccumulatedStatement("before AttributeEnd"); err != nil {
		return err
	}

	// Pop attribute block from stack and add to scene
	if len(p.attributeStack) > 0 {
		completedAttribute := p.attributeStack[len(p.attributeStack)-1]
		p.scene.Attributes = append(p.scene.Attributes, *completedAttribute)
		p.attributeStack = p.attributeStack[:len(p.attributeStack)-1]
	}

	// Pop graphics state from stack to restore previous state
	if len(p.stateStack) > 0 {
		restoredState := p.stateStack[len(p.stateStack)-1]
		p.currentMaterialIndex = restoredState.MaterialIndex
		p.stateStack = p.stateStack[:len(p.stateStack)-1]
	}
	return nil
}

// processLine processes a single line of PBRT input
func (p *PBRTParser) processLine(line string) error {
	line = strings.TrimSpace(line)

	// Skip empty lines and comments
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	// Handle special directives
	switch line {
	case "WorldBegin":
		return p.processWorldBegin()
	case "WorldEnd":
		return p.processWorldEnd()
	case "AttributeBegin":
		return p.processAttributeBegin()
	case "AttributeEnd":
		return p.processAttributeEnd()
	}

	// Check if this line starts a new statement or continues the previous one
	if isStatementStart(line) {
		// Process any accumulated statement first
		if err := p.processAccumulatedStatement(""); err != nil {
			return err
		}
		// Start new statement
		p.statementLines = []string{line}
	} else {
		// Continue previous statement
		if len(p.statementLines) == 0 {
			return fmt.Errorf("unexpected continuation line: %s", line)
		}
		p.statementLines = append(p.statementLines, line)
	}

	return nil
}

// finalize processes any remaining accumulated statements
func (p *PBRTParser) finalize() error {
	return p.processAccumulatedStatement("at end of file")
}

// routeStatement routes a parsed statement to the appropriate section of the scene
func (p *PBRTParser) routeStatement(stmt *PBRTStatement) error {
	// Handle special cases
	if stmt.Type == "LookAt" {
		if err := parseLookAt(stmt, p.scene); err != nil {
			return fmt.Errorf("error parsing LookAt: %v", err)
		}
		return nil
	}

	currentAttribute := p.getCurrentAttribute()

	// Route statement to appropriate section
	if currentAttribute != nil {
		// Inside AttributeBegin block - add to current attribute block
		switch stmt.Type {
		case "Material":
			currentAttribute.Materials = append(currentAttribute.Materials, *stmt)
			// Material definitions within attribute blocks don't affect global material index
		case "Shape":
			// For shapes within attribute blocks, use local material index within that block
			localMaterialIndex := len(currentAttribute.Materials) - 1
			if localMaterialIndex >= 0 {
				stmt.MaterialIndex = localMaterialIndex
			} else {
				// No materials defined in this attribute block, use global material index
				stmt.MaterialIndex = p.currentMaterialIndex
			}

			// Check if there's an active area light source in the current graphics state
			if len(p.stateStack) > 0 && p.stateStack[len(p.stateStack)-1].AreaLightSource != nil {
				areaLight := p.stateStack[len(p.stateStack)-1].AreaLightSource
				// Copy area light parameters to the shape
				if stmt.Parameters == nil {
					stmt.Parameters = make(map[string]PBRTParam)
				}
				// Add a special marker to indicate this shape is an area light
				stmt.Parameters["_areaLight"] = PBRTParam{Type: "bool", Values: []string{"true"}}
				// Copy emission parameters from area light to shape
				for paramName, param := range areaLight.Parameters {
					// Copy emission-related parameters
					if paramName == "L" || paramName == "power" {
						stmt.Parameters[paramName] = param
					}
				}
			}

			currentAttribute.Shapes = append(currentAttribute.Shapes, *stmt)
		case "LightSource":
			currentAttribute.LightSources = append(currentAttribute.LightSources, *stmt)
		case "AreaLightSource":
			// AreaLightSource modifies graphics state - it affects subsequent shapes
			if len(p.stateStack) > 0 {
				p.stateStack[len(p.stateStack)-1].AreaLightSource = stmt
			}
			// Also store it in the attribute block for processing
			currentAttribute.LightSources = append(currentAttribute.LightSources, *stmt)
		case "Translate", "Rotate", "Scale", "Transform":
			currentAttribute.Transforms = append(currentAttribute.Transforms, *stmt)
		}
	} else {
		// Global level - route to pre-world or world sections
		if !p.inWorld {
			// Pre-world statements
			switch stmt.Type {
			case "Camera":
				p.scene.Camera = stmt
			case "Film":
				p.scene.Film = stmt
			case "Sampler":
				p.scene.Sampler = stmt
			case "Integrator":
				p.scene.Integrator = stmt
			}
		} else {
			// World statements
			switch stmt.Type {
			case "Material":
				p.scene.Materials = append(p.scene.Materials, *stmt)
				p.currentMaterialIndex = len(p.scene.Materials) - 1 // Update current material index
			case "Shape":
				stmt.MaterialIndex = p.currentMaterialIndex // Assign current material to shape

				// Check if there's an active area light source in the current graphics state
				if len(p.stateStack) > 0 && p.stateStack[len(p.stateStack)-1].AreaLightSource != nil {
					areaLight := p.stateStack[len(p.stateStack)-1].AreaLightSource
					// Copy area light parameters to the shape
					if stmt.Parameters == nil {
						stmt.Parameters = make(map[string]PBRTParam)
					}
					// Add a special marker to indicate this shape is an area light
					stmt.Parameters["_areaLight"] = PBRTParam{Type: "bool", Values: []string{"true"}}
					// Copy emission parameters from area light to shape
					for paramName, param := range areaLight.Parameters {
						// Copy emission-related parameters
						if paramName == "L" || paramName == "power" {
							stmt.Parameters[paramName] = param
						}
					}
				}

				p.scene.Shapes = append(p.scene.Shapes, *stmt)
			case "LightSource":
				p.scene.LightSources = append(p.scene.LightSources, *stmt)
			case "AreaLightSource":
				// AreaLightSource at global level - set global area light state
				if len(p.stateStack) > 0 {
					p.stateStack[len(p.stateStack)-1].AreaLightSource = stmt
				}
				// Also store it for processing
				p.scene.LightSources = append(p.scene.LightSources, *stmt)
			case "Translate", "Rotate", "Scale", "Transform":
				p.scene.Transforms = append(p.scene.Transforms, *stmt)
			}
		}
	}
	return nil
}

// validateFilePath validates a file path for security issues
func validateFilePath(filename string) error {
	// Check for empty filename
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Clean the path to resolve . and .. components
	cleanPath := filepath.Clean(filename)

	// Only allow files in scenes/ directory or temp directory (for tests)
	// Also allow relative paths that resolve to scenes directory
	if !strings.HasPrefix(cleanPath, "scenes/") &&
		!strings.HasPrefix(cleanPath, os.TempDir()) &&
		!strings.Contains(cleanPath, "scenes/") {
		return fmt.Errorf("file path must be in scenes/ directory")
	}

	// Check for directory traversal attempts - but allow if it resolves to scenes/
	if strings.Contains(cleanPath, "..") {
		// Check if the final resolved path ends up in scenes directory
		if !strings.Contains(cleanPath, "scenes/") {
			return fmt.Errorf("invalid file path: directory traversal not allowed")
		}
	}

	// Check file extension (only allow .pbrt files)
	if !strings.HasSuffix(strings.ToLower(cleanPath), ".pbrt") {
		return fmt.Errorf("invalid file type: only .pbrt files are allowed")
	}

	// Check for extremely long paths that could cause issues
	if len(cleanPath) > 512 {
		return fmt.Errorf("file path too long: maximum 512 characters allowed")
	}

	// Check for null bytes (could indicate path manipulation)
	if strings.Contains(filename, "\x00") {
		return fmt.Errorf("invalid file path: null bytes not allowed")
	}

	return nil
}

// parseLookAt parses a LookAt statement into scene camera vectors
func parseLookAt(stmt *PBRTStatement, scene *PBRTScene) error {
	// LookAt should have 9 values: eyex eyey eyez atx aty atz upx upy upz
	if len(stmt.Parameters) != 1 || len(stmt.Parameters["values"].Values) != 9 {
		return fmt.Errorf("LookAt requires 9 values")
	}

	values := stmt.Parameters["values"].Values

	// Parse eye position
	eyeX, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		return fmt.Errorf("invalid eye X coordinate '%s': %v", values[0], err)
	}
	eyeY, err := strconv.ParseFloat(values[1], 64)
	if err != nil {
		return fmt.Errorf("invalid eye Y coordinate '%s': %v", values[1], err)
	}
	eyeZ, err := strconv.ParseFloat(values[2], 64)
	if err != nil {
		return fmt.Errorf("invalid eye Z coordinate '%s': %v", values[2], err)
	}
	scene.LookAt = &core.Vec3{X: eyeX, Y: eyeY, Z: eyeZ}

	// Parse look at target
	atX, err := strconv.ParseFloat(values[3], 64)
	if err != nil {
		return fmt.Errorf("invalid look-at X coordinate '%s': %v", values[3], err)
	}
	atY, err := strconv.ParseFloat(values[4], 64)
	if err != nil {
		return fmt.Errorf("invalid look-at Y coordinate '%s': %v", values[4], err)
	}
	atZ, err := strconv.ParseFloat(values[5], 64)
	if err != nil {
		return fmt.Errorf("invalid look-at Z coordinate '%s': %v", values[5], err)
	}
	scene.LookAtTo = &core.Vec3{X: atX, Y: atY, Z: atZ}

	// Parse up vector
	upX, err := strconv.ParseFloat(values[6], 64)
	if err != nil {
		return fmt.Errorf("invalid up X coordinate '%s': %v", values[6], err)
	}
	upY, err := strconv.ParseFloat(values[7], 64)
	if err != nil {
		return fmt.Errorf("invalid up Y coordinate '%s': %v", values[7], err)
	}
	upZ, err := strconv.ParseFloat(values[8], 64)
	if err != nil {
		return fmt.Errorf("invalid up Z coordinate '%s': %v", values[8], err)
	}
	scene.LookAtUp = &core.Vec3{X: upX, Y: upY, Z: upZ}

	return nil
}

// tokenizePBRT tokenizes a PBRT line respecting quoted strings and brackets
func tokenizePBRT(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	inBrackets := false

	for _, char := range line {
		switch char {
		case '"':
			if !inBrackets {
				current.WriteRune(char)
				if inQuotes {
					// End of quoted string
					tokens = append(tokens, current.String())
					current.Reset()
					inQuotes = false
				} else {
					// Start of quoted string
					inQuotes = true
				}
			} else {
				current.WriteRune(char)
			}
		case '[':
			if !inQuotes {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				current.WriteRune(char)
				inBrackets = true
			} else {
				current.WriteRune(char)
			}
		case ']':
			if !inQuotes && inBrackets {
				current.WriteRune(char)
				tokens = append(tokens, current.String())
				current.Reset()
				inBrackets = false
			} else {
				current.WriteRune(char)
			}
		case ' ', '\t':
			if inQuotes || inBrackets {
				current.WriteRune(char)
			} else {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(char)
		}
	}

	// Add final token if any
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseStatement parses a single PBRT statement line
func parseStatement(line string) (*PBRTStatement, error) {
	// Handle LookAt specially (has no quotes around type)
	if strings.HasPrefix(line, "LookAt") {
		parts := strings.Fields(line[6:]) // Skip "LookAt"
		stmt := &PBRTStatement{
			Type: "LookAt",
			Parameters: map[string]PBRTParam{
				"values": {Type: "float", Values: parts},
			},
		}
		return stmt, nil
	}

	// Handle other transform statements (Translate, Rotate, Scale)
	for _, transform := range []string{"Translate", "Rotate", "Scale", "Transform"} {
		if strings.HasPrefix(line, transform) {
			parts := strings.Fields(line[len(transform):])
			stmt := &PBRTStatement{
				Type: transform,
				Parameters: map[string]PBRTParam{
					"values": {Type: "float", Values: parts},
				},
			}
			return stmt, nil
		}
	}

	// Parse regular statements: Type "subtype" "param type" value
	parts := tokenizePBRT(line)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid statement format")
	}

	stmt := &PBRTStatement{
		Type:       parts[0],
		Parameters: make(map[string]PBRTParam),
	}

	// Extract subtype (quoted string after type)
	if len(parts) > 1 && strings.HasPrefix(parts[1], "\"") && strings.HasSuffix(parts[1], "\"") {
		stmt.Subtype = strings.Trim(parts[1], "\"")
		parts = parts[2:] // Skip type and subtype
	} else {
		parts = parts[1:] // Skip only type
	}

	// Parse parameters
	i := 0
	for i < len(parts) {
		if !strings.HasPrefix(parts[i], "\"") {
			i++
			continue
		}

		// Find parameter name and type
		paramDef := strings.Trim(parts[i], "\"")
		paramParts := strings.Fields(paramDef)
		if len(paramParts) != 2 {
			i++
			continue
		}

		paramType := paramParts[0]
		paramName := paramParts[1]
		i++

		// Parse parameter value(s)
		var values []string
		if i < len(parts) {
			if strings.HasPrefix(parts[i], "[") && strings.HasSuffix(parts[i], "]") {
				// Array value - already tokenized as single token
				arrayStr := strings.Trim(parts[i], "[] ")
				values = strings.Fields(arrayStr)
				i++
			} else {
				// Single value
				values = []string{parts[i]}
				i++
			}
		}

		stmt.Parameters[paramName] = PBRTParam{
			Type:   paramType,
			Values: values,
		}
	}

	return stmt, nil
}

// GetFloatParam extracts a float parameter from a PBRT statement
func (stmt *PBRTStatement) GetFloatParam(name string) (float64, bool) {
	param, exists := stmt.Parameters[name]
	if !exists || len(param.Values) == 0 {
		return 0, false
	}
	val, err := strconv.ParseFloat(param.Values[0], 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// GetRGBParam extracts an RGB color parameter from a PBRT statement
func (stmt *PBRTStatement) GetRGBParam(name string) (*core.Vec3, bool) {
	param, exists := stmt.Parameters[name]
	if !exists || len(param.Values) < 3 {
		return nil, false
	}
	r, err1 := strconv.ParseFloat(param.Values[0], 64)
	g, err2 := strconv.ParseFloat(param.Values[1], 64)
	b, err3 := strconv.ParseFloat(param.Values[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, false
	}
	return &core.Vec3{X: r, Y: g, Z: b}, true
}

// IsAreaLight checks if a shape statement is marked as an area light
func (stmt *PBRTStatement) IsAreaLight() bool {
	areaLightParam, exists := stmt.Parameters["_areaLight"]
	return exists && len(areaLightParam.Values) > 0 && areaLightParam.Values[0] == "true"
}

// GetPoint3Param extracts a point3 parameter from a PBRT statement
func (stmt *PBRTStatement) GetPoint3Param(name string) (*core.Vec3, bool) {
	param, exists := stmt.Parameters[name]
	if !exists || len(param.Values) < 3 {
		return nil, false
	}
	x, err1 := strconv.ParseFloat(param.Values[0], 64)
	y, err2 := strconv.ParseFloat(param.Values[1], 64)
	z, err3 := strconv.ParseFloat(param.Values[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, false
	}
	return &core.Vec3{X: x, Y: y, Z: z}, true
}

// GetStringParam extracts a string parameter from a PBRT statement
func (stmt *PBRTStatement) GetStringParam(name string) (string, bool) {
	param, exists := stmt.Parameters[name]
	if !exists || len(param.Values) == 0 {
		return "", false
	}
	return param.Values[0], true
}

// isStatementStart determines if a line starts a new PBRT statement
func isStatementStart(line string) bool {
	// A line starts a statement if it begins with a known PBRT directive
	statementTypes := []string{
		"Camera", "Film", "Sampler", "Integrator", "LookAt",
		"Material", "Shape", "LightSource", "AreaLightSource",
		"Translate", "Rotate", "Scale", "Transform",
		"ReverseOrientation", "Attribute",
	}

	for _, stmt := range statementTypes {
		if strings.HasPrefix(line, stmt+" ") || line == stmt {
			return true
		}
	}
	return false
}

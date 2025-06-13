package loaders

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// PLYHeader represents the parsed header information from a PLY file
type PLYHeader struct {
	Format      string // "binary_little_endian", "binary_big_endian", or "ascii"
	Version     string // Usually "1.0"
	VertexCount int
	FaceCount   int
	VertexProps []PLYProperty
	FaceProps   []PLYProperty

	// Property detection flags
	HasNormals    bool
	HasColors     bool
	HasTexCoords  bool
	HasQuality    bool
	HasConfidence bool
	HasIntensity  bool

	// Property indices for efficient access
	NormalIndices   [3]int // Indices of nx, ny, nz properties
	ColorIndices    [3]int // Indices of red, green, blue properties
	TexCoordIndices [2]int // Indices of u, v or s, t properties
	QualityIndex    int    // Index of quality property
	ConfidenceIndex int    // Index of confidence property
	IntensityIndex  int    // Index of intensity property
}

// PLYProperty represents a property definition in the PLY header
type PLYProperty struct {
	Name     string
	Type     string
	IsList   bool
	ListType string // For list properties, the type of the count
	DataType string // For list properties, the type of the data
}

// PLYData contains the raw data loaded from a PLY file
type PLYData struct {
	Vertices   []core.Vec3 // Vertex positions (x, y, z)
	Faces      []int       // Triangle indices (3 per triangle)
	Normals    []core.Vec3 // Per-vertex normals (nx, ny, nz) - empty if not present
	Colors     []core.Vec3 // Per-vertex colors (r, g, b) normalized to [0,1] - empty if not present
	TexCoords  []core.Vec2 // Per-vertex texture coordinates (u, v) - empty if not present
	Quality    []float64   // Per-vertex quality values - empty if not present
	Confidence []float64   // Per-vertex confidence values - empty if not present
	Intensity  []float64   // Per-vertex intensity values - empty if not present

	// Face properties
	FaceColors    []core.Vec3 // Per-face colors - empty if not present
	FaceMaterials []int       // Per-face material indices - empty if not present

	// Additional vertex properties (stored as generic float64 slices)
	CustomFloatProps map[string][]float64 // Custom float properties by name
	CustomIntProps   map[string][]int     // Custom integer properties by name
}

// LoadPLY loads a PLY file and returns the raw vertex and face data
func LoadPLY(filename string) (*PLYData, error) {
	startTime := time.Now()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PLY file: %v", err)
	}
	defer file.Close()

	// Parse header
	header, headerSize, err := parsePLYHeader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PLY header: %v", err)
	}

	// Seek to start of binary data
	_, err = file.Seek(int64(headerSize), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to binary data: %v", err)
	}

	// Read vertices and faces based on format
	var plyData *PLYData

	switch header.Format {
	case "binary_little_endian":
		plyData, err = readBinaryLittleEndianWithNormals(file, header)
	case "binary_big_endian":
		return nil, fmt.Errorf("binary big-endian PLY format not yet implemented")
	case "ascii":
		return nil, fmt.Errorf("ASCII PLY format not yet supported")
	default:
		return nil, fmt.Errorf("unsupported PLY format: %s", header.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read PLY data: %v", err)
	}

	fmt.Printf("✅ Loaded PLY data: %d vertices, %d triangles in %v\n",
		len(plyData.Vertices), len(plyData.Faces)/3, time.Since(startTime))

	return plyData, nil
}

// parsePLYHeader parses the PLY header and returns header info and the byte offset where binary data starts
func parsePLYHeader(file *os.File) (*PLYHeader, int, error) {
	header := &PLYHeader{
		VertexProps: make([]PLYProperty, 0),
		FaceProps:   make([]PLYProperty, 0),
	}

	scanner := bufio.NewScanner(file)
	var bytesRead int
	var currentElement string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		bytesRead += len(scanner.Bytes()) + 1 // +1 for newline

		if line == "end_header" {
			break
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "ply":
			// PLY magic number - already validated
		case "format":
			if len(parts) >= 3 {
				header.Format = parts[1]
				header.Version = parts[2]
			}
		case "comment":
			// Ignore comments
		case "element":
			if len(parts) >= 3 {
				elementType := parts[1]
				count, err := strconv.Atoi(parts[2])
				if err != nil {
					return nil, 0, fmt.Errorf("invalid element count: %s", parts[2])
				}

				currentElement = elementType
				switch elementType {
				case "vertex":
					header.VertexCount = count
				case "face":
					header.FaceCount = count
				}
			}
		case "property":
			prop, err := parsePLYProperty(parts[1:])
			if err != nil {
				return nil, 0, fmt.Errorf("failed to parse property: %v", err)
			}

			switch currentElement {
			case "vertex":
				header.VertexProps = append(header.VertexProps, prop)
				propIndex := len(header.VertexProps) - 1

				// Check for normal properties
				switch prop.Name {
				case "nx":
					header.HasNormals = true
					header.NormalIndices[0] = propIndex
				case "ny":
					header.HasNormals = true
					header.NormalIndices[1] = propIndex
				case "nz":
					header.HasNormals = true
					header.NormalIndices[2] = propIndex

				// Check for color properties
				case "red", "r":
					header.HasColors = true
					header.ColorIndices[0] = propIndex
				case "green", "g":
					header.HasColors = true
					header.ColorIndices[1] = propIndex
				case "blue", "b":
					header.HasColors = true
					header.ColorIndices[2] = propIndex

				// Check for texture coordinate properties
				case "u", "s", "texture_u":
					header.HasTexCoords = true
					header.TexCoordIndices[0] = propIndex
				case "v", "t", "texture_v":
					header.HasTexCoords = true
					header.TexCoordIndices[1] = propIndex

				// Check for other common properties
				case "quality":
					header.HasQuality = true
					header.QualityIndex = propIndex
				case "confidence":
					header.HasConfidence = true
					header.ConfidenceIndex = propIndex
				case "intensity":
					header.HasIntensity = true
					header.IntensityIndex = propIndex
				}
			case "face":
				header.FaceProps = append(header.FaceProps, prop)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("error reading header: %v", err)
	}

	return header, bytesRead, nil
}

// parsePLYProperty parses a property line from the PLY header
func parsePLYProperty(parts []string) (PLYProperty, error) {
	if len(parts) < 2 {
		return PLYProperty{}, fmt.Errorf("invalid property definition")
	}

	prop := PLYProperty{}

	if parts[0] == "list" {
		if len(parts) < 4 {
			return PLYProperty{}, fmt.Errorf("invalid list property definition")
		}
		prop.IsList = true
		prop.ListType = parts[1]
		prop.DataType = parts[2]
		prop.Name = parts[3]
	} else {
		prop.Type = parts[0]
		prop.Name = parts[1]
	}

	return prop, nil
}

// readBinaryLittleEndianWithNormals reads binary little-endian PLY data with all properties
func readBinaryLittleEndianWithNormals(file *os.File, header *PLYHeader) (*PLYData, error) {
	// Pre-allocate slices with exact capacity to avoid reallocations
	vertices := make([]core.Vec3, 0, header.VertexCount)
	faces := make([]int, 0, header.FaceCount*3) // Assuming triangular faces

	var normals []core.Vec3
	var colors []core.Vec3
	var texCoords []core.Vec2
	var quality []float64
	var confidence []float64
	var intensity []float64

	if header.HasNormals {
		normals = make([]core.Vec3, 0, header.VertexCount)
	}
	if header.HasColors {
		colors = make([]core.Vec3, 0, header.VertexCount)
	}
	if header.HasTexCoords {
		texCoords = make([]core.Vec2, 0, header.VertexCount)
	}
	if header.HasQuality {
		quality = make([]float64, 0, header.VertexCount)
	}
	if header.HasConfidence {
		confidence = make([]float64, 0, header.VertexCount)
	}
	if header.HasIntensity {
		intensity = make([]float64, 0, header.VertexCount)
	}

	// Read vertices using optimized bulk approach
	// Calculate vertex size and read all vertex data at once
	vertexSize := calculateVertexSize(header.VertexProps)
	totalVertexBytes := vertexSize * header.VertexCount
	vertexData := make([]byte, totalVertexBytes)
	_, err := io.ReadFull(file, vertexData)
	if err != nil {
		return nil, fmt.Errorf("failed to read vertex data: %v", err)
	}

	// Parse vertices from bulk data
	for i := 0; i < header.VertexCount; i++ {
		offset := i * vertexSize
		vertex := parseVertexFromBytes(vertexData[offset:offset+vertexSize], header.VertexProps)

		vertices = append(vertices, core.NewVec3(float64(vertex.X), float64(vertex.Y), float64(vertex.Z)))

		if header.HasNormals {
			normals = append(normals, core.NewVec3(float64(vertex.NX), float64(vertex.NY), float64(vertex.NZ)))
		}

		if header.HasColors {
			// Convert from 0-255 to 0-1 range
			colors = append(colors, core.NewVec3(
				float64(vertex.R)/255.0,
				float64(vertex.G)/255.0,
				float64(vertex.B)/255.0,
			))
		}

		if header.HasTexCoords {
			texCoords = append(texCoords, core.NewVec2(float64(vertex.U), float64(vertex.V)))
		}

		if header.HasQuality {
			quality = append(quality, float64(vertex.Quality))
		}

		if header.HasConfidence {
			confidence = append(confidence, float64(vertex.Confidence))
		}

		if header.HasIntensity {
			intensity = append(intensity, float64(vertex.Intensity))
		}
	}

	// Read faces with buffered approach for better I/O performance

	// Create buffered reader for more efficient I/O
	bufReader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer

	for i := 0; i < header.FaceCount; i++ {

		// Read face data efficiently using buffered reader
		for j, prop := range header.FaceProps {
			if prop.IsList && prop.Name == "vertex_indices" {
				// Read count based on the actual list type from header
				var vertexCount int
				switch prop.ListType {
				case "uchar", "uint8":
					var count uint8
					if err := binary.Read(bufReader, binary.LittleEndian, &count); err != nil {
						return nil, fmt.Errorf("failed to read face vertex count (uchar) at face %d: %v", i, err)
					}
					vertexCount = int(count)
				case "int", "int32":
					var count int32
					if err := binary.Read(bufReader, binary.LittleEndian, &count); err != nil {
						return nil, fmt.Errorf("failed to read face vertex count (int32) at face %d: %v", i, err)
					}
					vertexCount = int(count)
				default:
					return nil, fmt.Errorf("unsupported list count type: %s", prop.ListType)
				}

				if vertexCount != 3 {
					return nil, fmt.Errorf("only triangular faces supported, got %d vertices at face %d", vertexCount, i)
				}

				// Read indices based on the data type
				var indices [3]int
				switch prop.DataType {
				case "int", "int32":
					var indexBuffer [3]int32
					if err := binary.Read(bufReader, binary.LittleEndian, &indexBuffer); err != nil {
						return nil, fmt.Errorf("failed to read face indices (int32) at face %d: %v", i, err)
					}
					indices[0] = int(indexBuffer[0])
					indices[1] = int(indexBuffer[1])
					indices[2] = int(indexBuffer[2])
				case "uint", "uint32":
					var indexBuffer [3]uint32
					if err := binary.Read(bufReader, binary.LittleEndian, &indexBuffer); err != nil {
						return nil, fmt.Errorf("failed to read face indices (uint32) at face %d: %v", i, err)
					}
					indices[0] = int(indexBuffer[0])
					indices[1] = int(indexBuffer[1])
					indices[2] = int(indexBuffer[2])
				default:
					return nil, fmt.Errorf("unsupported face index data type: %s", prop.DataType)
				}

				// Append to faces slice
				faces = append(faces, indices[0], indices[1], indices[2])
			} else {
				// Skip unknown face properties using buffered reader
				if err := skipPropertyBuffered(bufReader, prop); err != nil {
					return nil, fmt.Errorf("failed to skip face property %s at face %d, prop %d: %v", prop.Name, i, j, err)
				}
			}
		}
	}

	return &PLYData{
		Vertices:         vertices,
		Faces:            faces,
		Normals:          normals,
		Colors:           colors,
		TexCoords:        texCoords,
		Quality:          quality,
		Confidence:       confidence,
		Intensity:        intensity,
		CustomFloatProps: make(map[string][]float64),
		CustomIntProps:   make(map[string][]int),
	}, nil
}

// skipPropertyBuffered skips a property in the buffered binary stream
func skipPropertyBuffered(reader *bufio.Reader, prop PLYProperty) error {
	if prop.IsList {
		// Read list count
		var count uint8
		switch prop.ListType {
		case "uchar", "uint8":
			if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported list count type: %s", prop.ListType)
		}

		// Skip list elements
		for i := 0; i < int(count); i++ {
			if err := skipSimpleTypeBuffered(reader, prop.DataType); err != nil {
				return err
			}
		}
	} else {
		return skipSimpleTypeBuffered(reader, prop.Type)
	}
	return nil
}

// skipSimpleTypeBuffered skips a simple data type in the buffered binary stream
func skipSimpleTypeBuffered(reader *bufio.Reader, dataType string) error {
	switch dataType {
	case "float", "float32":
		var dummy float32
		return binary.Read(reader, binary.LittleEndian, &dummy)
	case "double", "float64":
		var dummy float64
		return binary.Read(reader, binary.LittleEndian, &dummy)
	case "int", "int32":
		var dummy int32
		return binary.Read(reader, binary.LittleEndian, &dummy)
	case "uint", "uint32":
		var dummy uint32
		return binary.Read(reader, binary.LittleEndian, &dummy)
	case "short", "int16":
		var dummy int16
		return binary.Read(reader, binary.LittleEndian, &dummy)
	case "ushort", "uint16":
		var dummy uint16
		return binary.Read(reader, binary.LittleEndian, &dummy)
	case "char", "int8":
		var dummy int8
		return binary.Read(reader, binary.LittleEndian, &dummy)
	case "uchar", "uint8":
		var dummy uint8
		return binary.Read(reader, binary.LittleEndian, &dummy)
	default:
		return fmt.Errorf("unsupported data type: %s", dataType)
	}
}

// skipProperty skips a property in the binary stream
func skipProperty(file *os.File, prop PLYProperty, byteOrder binary.ByteOrder) error {
	if prop.IsList {
		// Read list count
		var count uint8
		switch prop.ListType {
		case "uchar", "uint8":
			if err := binary.Read(file, byteOrder, &count); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported list count type: %s", prop.ListType)
		}

		// Skip list elements
		for i := 0; i < int(count); i++ {
			if err := skipSimpleType(file, prop.DataType, byteOrder); err != nil {
				return err
			}
		}
	} else {
		return skipSimpleType(file, prop.Type, byteOrder)
	}
	return nil
}

// skipSimpleType skips a simple data type in the binary stream
func skipSimpleType(file *os.File, dataType string, byteOrder binary.ByteOrder) error {
	switch dataType {
	case "float", "float32":
		var dummy float32
		return binary.Read(file, byteOrder, &dummy)
	case "double", "float64":
		var dummy float64
		return binary.Read(file, byteOrder, &dummy)
	case "int", "int32":
		var dummy int32
		return binary.Read(file, byteOrder, &dummy)
	case "uint", "uint32":
		var dummy uint32
		return binary.Read(file, byteOrder, &dummy)
	case "short", "int16":
		var dummy int16
		return binary.Read(file, byteOrder, &dummy)
	case "ushort", "uint16":
		var dummy uint16
		return binary.Read(file, byteOrder, &dummy)
	case "char", "int8":
		var dummy int8
		return binary.Read(file, byteOrder, &dummy)
	case "uchar", "uint8":
		var dummy uint8
		return binary.Read(file, byteOrder, &dummy)
	default:
		return fmt.Errorf("unsupported data type: %s", dataType)
	}
}

// calculateVertexSize calculates the size in bytes of a single vertex
func calculateVertexSize(props []PLYProperty) int {
	size := 0
	for _, prop := range props {
		if prop.IsList {
			// Lists are variable size, can't pre-calculate
			continue
		}
		size += getTypeSize(prop.Type)
	}
	return size
}

// getTypeSize returns the size in bytes of a PLY data type
func getTypeSize(dataType string) int {
	switch dataType {
	case "float", "float32", "int", "int32", "uint", "uint32":
		return 4
	case "double", "float64":
		return 8
	case "short", "int16", "ushort", "uint16":
		return 2
	case "char", "int8", "uchar", "uint8":
		return 1
	default:
		return 4 // Default to 4 bytes
	}
}

// VertexData holds all possible vertex properties
type VertexData struct {
	X, Y, Z             float32
	NX, NY, NZ          float32
	R, G, B             uint8
	U, V                float32
	Quality, Confidence float32
	Intensity           float32
	CustomFloats        map[string]float32
	CustomInts          map[string]int32
}

// parseVertexFromBytes extracts all vertex data from a byte slice
func parseVertexFromBytes(data []byte, props []PLYProperty) VertexData {
	vertex := VertexData{
		CustomFloats: make(map[string]float32),
		CustomInts:   make(map[string]int32),
	}

	offset := 0
	for _, prop := range props {
		if prop.IsList {
			// Skip list properties in vertex data (shouldn't happen normally)
			continue
		}

		size := getTypeSize(prop.Type)
		if offset+size > len(data) {
			break
		}

		buf := bytes.NewReader(data[offset : offset+size])

		switch prop.Type {
		case "float", "float32":
			var value float32
			if binary.Read(buf, binary.LittleEndian, &value) == nil {
				switch prop.Name {
				case "x":
					vertex.X = value
				case "y":
					vertex.Y = value
				case "z":
					vertex.Z = value
				case "nx":
					vertex.NX = value
				case "ny":
					vertex.NY = value
				case "nz":
					vertex.NZ = value
				case "u", "s", "texture_u":
					vertex.U = value
				case "v", "t", "texture_v":
					vertex.V = value
				case "quality":
					vertex.Quality = value
				case "confidence":
					vertex.Confidence = value
				case "intensity":
					vertex.Intensity = value
				default:
					vertex.CustomFloats[prop.Name] = value
				}
			}
		case "uchar", "uint8":
			var value uint8
			if binary.Read(buf, binary.LittleEndian, &value) == nil {
				switch prop.Name {
				case "red", "r":
					vertex.R = value
				case "green", "g":
					vertex.G = value
				case "blue", "b":
					vertex.B = value
				}
			}
		case "int", "int32":
			var value int32
			if binary.Read(buf, binary.LittleEndian, &value) == nil {
				vertex.CustomInts[prop.Name] = value
			}
		case "uint", "uint32":
			var value uint32
			if binary.Read(buf, binary.LittleEndian, &value) == nil {
				vertex.CustomInts[prop.Name] = int32(value) // Convert to signed for simplicity
			}
		case "short", "int16":
			var value int16
			if binary.Read(buf, binary.LittleEndian, &value) == nil {
				vertex.CustomInts[prop.Name] = int32(value)
			}
		case "ushort", "uint16":
			var value uint16
			if binary.Read(buf, binary.LittleEndian, &value) == nil {
				vertex.CustomInts[prop.Name] = int32(value)
			}
		case "double", "float64":
			var value float64
			if binary.Read(buf, binary.LittleEndian, &value) == nil {
				vertex.CustomFloats[prop.Name] = float32(value) // Convert to float32 for simplicity
			}
		}

		offset += size
	}

	return vertex
}

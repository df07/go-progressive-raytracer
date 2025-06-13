package loaders

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
)

// PLYHeader represents the parsed header information from a PLY file
type PLYHeader struct {
	Format        string // "binary_little_endian", "binary_big_endian", or "ascii"
	Version       string // Usually "1.0"
	VertexCount   int
	FaceCount     int
	VertexProps   []PLYProperty
	FaceProps     []PLYProperty
	HasNormals    bool
	NormalIndices [3]int // Indices of nx, ny, nz properties in vertex properties
}

// PLYProperty represents a property definition in the PLY header
type PLYProperty struct {
	Name     string
	Type     string
	IsList   bool
	ListType string // For list properties, the type of the count
	DataType string // For list properties, the type of the data
}

// LoadPLYMesh loads a PLY file and returns a TriangleMesh
func LoadPLYMesh(filename string, material core.Material) (*geometry.TriangleMesh, error) {
	startTime := time.Now()
	fmt.Printf("🔄 Opening PLY file: %s\n", filename)

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PLY file: %v", err)
	}
	defer file.Close()

	// Parse header
	fmt.Printf("🔄 Parsing PLY header...\n")
	headerStart := time.Now()
	header, headerSize, err := parsePLYHeader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PLY header: %v", err)
	}
	fmt.Printf("✅ Header parsed in %v (vertices: %d, faces: %d)\n",
		time.Since(headerStart), header.VertexCount, header.FaceCount)

	// Seek to start of binary data
	_, err = file.Seek(int64(headerSize), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to binary data: %v", err)
	}

	// Read vertices and faces based on format
	fmt.Printf("🔄 Reading binary data (%s format)...\n", header.Format)
	readStart := time.Now()

	var vertices []core.Vec3
	var faces []int
	var normals []core.Vec3

	switch header.Format {
	case "binary_little_endian":
		vertices, faces, normals, err = readBinaryLittleEndianWithNormals(file, header)
	case "binary_big_endian":
		vertices, faces, normals, err = readBinaryBigEndianWithNormals(file, header)
	case "ascii":
		return nil, fmt.Errorf("ASCII PLY format not yet supported")
	default:
		return nil, fmt.Errorf("unsupported PLY format: %s", header.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read PLY data: %v", err)
	}

	fmt.Printf("✅ Binary data read in %v (vertices: %d, triangles: %d, normals: %d)\n",
		time.Since(readStart), len(vertices), len(faces)/3, len(normals))

	// Create triangle mesh
	fmt.Printf("🔄 Building triangle mesh with BVH...\n")
	meshStart := time.Now()

	var mesh *geometry.TriangleMesh
	if len(normals) > 0 {
		mesh = geometry.NewTriangleMesh(vertices, faces, material, &geometry.TriangleMeshOptions{
			Normals: normals,
		})
	} else {
		mesh = geometry.NewTriangleMesh(vertices, faces, material, nil)
	}

	fmt.Printf("✅ Triangle mesh built in %v\n", time.Since(meshStart))
	fmt.Printf("🎉 Total PLY loading time: %v\n", time.Since(startTime))

	return mesh, nil
}

// LoadPLYMeshWithRotation loads a PLY file and returns a TriangleMesh with rotation applied
func LoadPLYMeshWithRotation(filename string, material core.Material, center, rotation core.Vec3) (*geometry.TriangleMesh, error) {
	startTime := time.Now()
	fmt.Printf("🔄 Opening PLY file: %s (with rotation)\n", filename)

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PLY file: %v", err)
	}
	defer file.Close()

	// Parse header
	fmt.Printf("🔄 Parsing PLY header...\n")
	headerStart := time.Now()
	header, headerSize, err := parsePLYHeader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PLY header: %v", err)
	}
	fmt.Printf("✅ Header parsed in %v (vertices: %d, faces: %d)\n",
		time.Since(headerStart), header.VertexCount, header.FaceCount)

	// Seek to start of binary data
	_, err = file.Seek(int64(headerSize), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to binary data: %v", err)
	}

	// Read vertices and faces based on format
	fmt.Printf("🔄 Reading binary data (%s format)...\n", header.Format)
	readStart := time.Now()

	var vertices []core.Vec3
	var faces []int
	var normals []core.Vec3

	switch header.Format {
	case "binary_little_endian":
		vertices, faces, normals, err = readBinaryLittleEndianWithNormals(file, header)
	case "binary_big_endian":
		vertices, faces, normals, err = readBinaryBigEndianWithNormals(file, header)
	case "ascii":
		return nil, fmt.Errorf("ASCII PLY format not yet supported")
	default:
		return nil, fmt.Errorf("unsupported PLY format: %s", header.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read PLY data: %v", err)
	}

	fmt.Printf("✅ Binary data read in %v (vertices: %d, triangles: %d, normals: %d)\n",
		time.Since(readStart), len(vertices), len(faces)/3, len(normals))

	// Create triangle mesh with rotation
	fmt.Printf("🔄 Building triangle mesh with rotation and BVH...\n")
	meshStart := time.Now()

	var mesh *geometry.TriangleMesh
	if len(normals) > 0 {
		// Apply rotation to vertices and use custom normals
		// TODO: Could also rotate the stored normals
		mesh = geometry.NewTriangleMesh(vertices, faces, material, &geometry.TriangleMeshOptions{
			Normals:  normals,
			Rotation: &rotation,
			Center:   &center,
		})
	} else {
		mesh = geometry.NewTriangleMesh(vertices, faces, material, &geometry.TriangleMeshOptions{
			Rotation: &rotation,
			Center:   &center,
		})
	}

	fmt.Printf("✅ Triangle mesh with rotation built in %v\n", time.Since(meshStart))
	fmt.Printf("🎉 Total PLY loading time: %v\n", time.Since(startTime))

	return mesh, nil
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
				// Check for normal properties
				if prop.Name == "nx" || prop.Name == "ny" || prop.Name == "nz" {
					header.HasNormals = true
					switch prop.Name {
					case "nx":
						header.NormalIndices[0] = len(header.VertexProps) - 1
					case "ny":
						header.NormalIndices[1] = len(header.VertexProps) - 1
					case "nz":
						header.NormalIndices[2] = len(header.VertexProps) - 1
					}
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

// readBinaryLittleEndianWithNormals reads binary little-endian PLY data with optimized face reading
func readBinaryLittleEndianWithNormals(file *os.File, header *PLYHeader) ([]core.Vec3, []int, []core.Vec3, error) {
	// Pre-allocate slices with exact capacity to avoid reallocations
	vertices := make([]core.Vec3, 0, header.VertexCount)
	faces := make([]int, 0, header.FaceCount*3) // Assuming triangular faces
	var normals []core.Vec3
	if header.HasNormals {
		normals = make([]core.Vec3, 0, header.VertexCount)
	}

	// Read vertices
	fmt.Printf("🔄 Reading %d vertices...\n", header.VertexCount)
	vertexStart := time.Now()
	progressInterval := header.VertexCount / 10 // 10% intervals
	if progressInterval == 0 {
		progressInterval = 1
	}

	for i := 0; i < header.VertexCount; i++ {
		if i%progressInterval == 0 {
			progress := float64(i) / float64(header.VertexCount) * 100
			fmt.Printf("   📊 Vertex progress: %.1f%% (%d/%d)\n", progress, i, header.VertexCount)
		}

		var x, y, z float32
		var nx, ny, nz float32

		for j, prop := range header.VertexProps {
			switch prop.Name {
			case "x":
				if err := binary.Read(file, binary.LittleEndian, &x); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to read vertex x: %v", err)
				}
			case "y":
				if err := binary.Read(file, binary.LittleEndian, &y); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to read vertex y: %v", err)
				}
			case "z":
				if err := binary.Read(file, binary.LittleEndian, &z); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to read vertex z: %v", err)
				}
			case "nx":
				if err := binary.Read(file, binary.LittleEndian, &nx); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to read normal nx: %v", err)
				}
			case "ny":
				if err := binary.Read(file, binary.LittleEndian, &ny); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to read normal ny: %v", err)
				}
			case "nz":
				if err := binary.Read(file, binary.LittleEndian, &nz); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to read normal nz: %v", err)
				}
			default:
				// Skip unknown properties
				if err := skipProperty(file, prop, binary.LittleEndian); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to skip vertex property %s at vertex %d, prop %d: %v", prop.Name, i, j, err)
				}
			}
		}

		vertices = append(vertices, core.NewVec3(float64(x), float64(y), float64(z)))
		if header.HasNormals {
			normals = append(normals, core.NewVec3(float64(nx), float64(ny), float64(nz)))
		}
	}
	fmt.Printf("✅ Vertices read in %v\n", time.Since(vertexStart))

	// Read faces with optimized approach
	fmt.Printf("🔄 Reading %d faces...\n", header.FaceCount)
	faceStart := time.Now()
	faceProgressInterval := header.FaceCount / 10 // 10% intervals
	if faceProgressInterval == 0 {
		faceProgressInterval = 1
	}

	for i := 0; i < header.FaceCount; i++ {
		if i%faceProgressInterval == 0 {
			progress := float64(i) / float64(header.FaceCount) * 100
			fmt.Printf("   📊 Face progress: %.1f%% (%d/%d)\n", progress, i, header.FaceCount)
		}

		// Read face data efficiently
		for j, prop := range header.FaceProps {
			if prop.IsList && prop.Name == "vertex_indices" {
				// Read count based on the actual list type from header
				var vertexCount int
				switch prop.ListType {
				case "uchar", "uint8":
					var count uint8
					if err := binary.Read(file, binary.LittleEndian, &count); err != nil {
						return nil, nil, nil, fmt.Errorf("failed to read face vertex count (uchar) at face %d: %v", i, err)
					}
					vertexCount = int(count)
				case "int", "int32":
					var count int32
					if err := binary.Read(file, binary.LittleEndian, &count); err != nil {
						return nil, nil, nil, fmt.Errorf("failed to read face vertex count (int32) at face %d: %v", i, err)
					}
					vertexCount = int(count)
				default:
					return nil, nil, nil, fmt.Errorf("unsupported list count type: %s", prop.ListType)
				}

				if vertexCount != 3 {
					return nil, nil, nil, fmt.Errorf("only triangular faces supported, got %d vertices at face %d", vertexCount, i)
				}

				// Read indices based on the data type
				var indices [3]int
				switch prop.DataType {
				case "int", "int32":
					var indexBuffer [3]int32
					if err := binary.Read(file, binary.LittleEndian, &indexBuffer); err != nil {
						return nil, nil, nil, fmt.Errorf("failed to read face indices (int32) at face %d: %v", i, err)
					}
					indices[0] = int(indexBuffer[0])
					indices[1] = int(indexBuffer[1])
					indices[2] = int(indexBuffer[2])
				case "uint", "uint32":
					var indexBuffer [3]uint32
					if err := binary.Read(file, binary.LittleEndian, &indexBuffer); err != nil {
						return nil, nil, nil, fmt.Errorf("failed to read face indices (uint32) at face %d: %v", i, err)
					}
					indices[0] = int(indexBuffer[0])
					indices[1] = int(indexBuffer[1])
					indices[2] = int(indexBuffer[2])
				default:
					return nil, nil, nil, fmt.Errorf("unsupported face index data type: %s", prop.DataType)
				}

				// Append to faces slice
				faces = append(faces, indices[0], indices[1], indices[2])
			} else {
				// Skip unknown face properties
				if err := skipProperty(file, prop, binary.LittleEndian); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to skip face property %s at face %d, prop %d: %v", prop.Name, i, j, err)
				}
			}
		}
	}
	fmt.Printf("✅ Faces read in %v\n", time.Since(faceStart))

	return vertices, faces, normals, nil
}

// readBinaryBigEndianWithNormals reads binary big-endian PLY data (placeholder)
func readBinaryBigEndianWithNormals(file *os.File, header *PLYHeader) ([]core.Vec3, []int, []core.Vec3, error) {
	// TODO: Implement big-endian reading if needed
	return nil, nil, nil, fmt.Errorf("binary big-endian PLY format not yet implemented")
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

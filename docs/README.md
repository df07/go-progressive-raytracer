# Documentation

## Architecture Documentation

Comprehensive documentation of the raytracer's design and implementation.

**Location**: [`/docs/architecture/`](architecture/)

**Contents**:
- [Core Types and Interfaces](architecture/core-types.md) - Vec3, Ray, SurfaceInteraction, Sampler
- [Material System](architecture/material-system.md) - BRDF interface, Lambertian, Metal, Dielectric
- [Geometry Primitives](architecture/geometry-primitives.md) - Sphere, Quad, Triangle, BVH acceleration
- [Scene System](architecture/scene-system.md) - Scene definition, asset loading, configuration

**Start here**: Read [architecture/README.md](architecture/README.md) for an overview and quick reference.

## Quick Links

**Getting Started**: See [CLAUDE.md](../CLAUDE.md) in the repository root for:
- Build and development commands
- Architecture overview
- Integrator and scene descriptions
- CLI usage examples
- Web interface details

**API Documentation**: GoDoc comments in source files provide detailed API documentation.

**Examples**: See `pkg/scene/*.go` for complete scene examples demonstrating all major features.

## Documentation Organization

```
docs/
├── README.md                          # This file
├── architecture/                      # Architecture documentation
│   ├── README.md                      # Architecture overview
│   ├── core-types.md                  # Foundation types
│   ├── material-system.md             # Material interface and types
│   ├── geometry-primitives.md         # Shapes and acceleration
│   └── scene-system.md                # Scene definition patterns
└── documentation-requests.md          # Internal: documentation request tracking
```

## Contributing to Documentation

Documentation should be:
- **Accurate**: Verified against source code
- **Specific**: File names, function signatures, concrete examples
- **Focused**: One topic per document
- **Current**: Describe what exists, not what's planned
- **Scannable**: Bullet points, headers, code blocks

When modifying the codebase, update relevant documentation in `/docs/architecture/`.

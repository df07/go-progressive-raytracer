# Documentation

## Directory Overview

### [Architecture](architecture/)

System architecture and component design.

**Key documents**:
- [Architecture Overview](architecture/README.md) - Start here for overall system design
- [Material System](architecture/material-system.md) - BRDF interface and material types
- [Texture System](architecture/texture-system.md) - Texture mapping and ColorSource interface
- [Integrator System](architecture/integrator-system.md) - Path tracing and BDPT algorithms
- [Rendering Pipeline](architecture/rendering-pipeline.md) - Progressive rendering flow

### [Guides](guides/)

Practical guides for testing, debugging, and development workflows.

**Key documents**:
- [Guide Overview](guides/README.md) - Navigation for all guides
- [CLI Usage](guides/cli-usage.md) - Command-line reference and testing workflows
- [Testing Strategy](guides/testing-strategy.md) - Three-level testing approach
- [Debugging Guide](guides/debugging-rendering-issues.md) - Systematic debugging workflows
- [Common Bug Patterns](guides/common-bug-patterns.md) - Catalog of recurring bugs and fixes

### [Implementation](implementation/)

Detailed algorithm implementation walkthroughs.

**Key documents**:
- [BDPT Implementation Guide](implementation/bdpt-implementation-guide.md) - Complete BDPT walkthrough

### [Specs](specs/)

Technical specifications and design decisions.

**Key documents**:
- [Texture Mapping Spec](specs/texture-mapping-spec.md) - Texture mapping specification

## Common Tasks

### I want to...

**Understand the overall architecture**
- Start: [Architecture Overview](architecture/README.md)
- Then: [Rendering Pipeline](architecture/rendering-pipeline.md)

**Debug rendering issues**
- Start: [Common Bug Patterns](guides/common-bug-patterns.md) to classify the bug
- Then: [Debugging Guide](guides/debugging-rendering-issues.md) for systematic approach
- Use: [CLI Usage](guides/cli-usage.md) for comparison renders

**Work with materials or textures**
- Start: [Material System](architecture/material-system.md)
- Then: [Texture System](architecture/texture-system.md)
- Debug: [Material Texture Data Flow](architecture/material-texture-data-flow.md)

**Modify BDPT**
- Start: [BDPT Implementation Guide](implementation/bdpt-implementation-guide.md)
- Test: [Testing Strategy](guides/testing-strategy.md)
- Debug: [Common Bug Patterns](guides/common-bug-patterns.md) (integrator inconsistencies)

**Write tests**
- Start: [Testing Strategy](guides/testing-strategy.md)
- Use: [CLI Usage](guides/cli-usage.md) for manual testing

**Understand UV coordinate flow**
- Read: [Material Texture Data Flow](architecture/material-texture-data-flow.md)

**Add a new scene**
- Start: [Scene System](architecture/scene-system.md)
- Examples: `pkg/scene/*.go`

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
├── architecture/                      # System architecture
│   ├── README.md                      # Architecture overview
│   ├── core-types.md                  # Foundation types
│   ├── material-system.md             # Material interface and types
│   ├── texture-system.md              # Texture mapping system
│   ├── geometry-primitives.md         # Shapes and acceleration
│   ├── scene-system.md                # Scene definition patterns
│   ├── integrator-system.md           # Light transport algorithms
│   ├── rendering-pipeline.md          # Progressive rendering pipeline
│   └── material-texture-data-flow.md  # UV coordinate flow
├── guides/                            # Testing and debugging
│   ├── README.md                      # Guide overview
│   ├── cli-usage.md                   # CLI reference and workflows
│   ├── testing-strategy.md            # Testing approach
│   ├── debugging-rendering-issues.md  # Debugging workflows
│   └── common-bug-patterns.md         # Bug pattern catalog
├── implementation/                    # Algorithm implementations
│   ├── README.md                      # Implementation guide overview
│   └── bdpt-implementation-guide.md   # BDPT detailed walkthrough
├── specs/                             # Specifications
│   ├── texture-mapping-spec.md        # Texture mapping specification
│   └── texture-mapping-code-review.md # Code review notes
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

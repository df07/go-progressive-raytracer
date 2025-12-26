# Documentation Requests

## Testing Strategy for Integrators
- **Requested**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **Requested by**: Agent debugging texture inconsistency bug
- **Reason**: No documentation exists on how to test integrators or compare PT vs BDPT
- **Current state**: None
- **Needed information**:
  - Overview of testing levels: unit tests (single rays), integration tests (luminance comparison), visual tests
  - How to use progressive_integration_test.go pattern for new integrator tests
  - When bugs appear at different levels (unit vs full render) and why
  - Standard debugging techniques: luminance comparison, visual diff, specific test scenes
  - How to write effective integrator comparison tests (PT vs BDPT should produce similar results)

## CLI Usage and Testing Workflow
- **Requested**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **Requested by**: Agent debugging texture inconsistency bug
- **Reason**: Unclear how to run renders for testing, what flags exist, where output goes
- **Current state**: Partial (CLAUDE.md has examples but not comprehensive)
- **Needed information**:
  - Complete CLI flag reference (what exists, what doesn't - e.g., no --output flag)
  - Where renders are saved (output/<scene>/render_<timestamp>.png)
  - How to do quick test renders (--max-samples, --max-passes for fast iteration)
  - How to compare two renders (PT vs BDPT) for debugging
  - Typical debugging workflow: modify code → build → render → compare

## Rendering Pipeline Architecture
- **Requested**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **Requested by**: Agent debugging texture inconsistency bug
- **Reason**: Relationship between integrators, renderers, tiles, and splats was unclear
- **Current state**: None
- **Needed information**:
  - High-level pipeline: Camera → Tiles → Integrator → Pixels
  - How progressive rendering works (multiple passes, tile-based parallelism)
  - What happens in each component (Renderer vs Integrator responsibilities)
  - BDPT-specific: splat system and when it's used
  - Why some bugs only appear in full renders vs unit tests
  - Diagram showing data flow from camera ray to final pixel

## Debugging Guide for Rendering Issues
- **Requested**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **Requested by**: Agent debugging texture inconsistency bug
- **Reason**: No guide exists for debugging common rendering problems
- **Current state**: None
- **Needed information**:
  - Common bug classes: brightness differences, color bleeding, artifacts, crashes
  - How to diagnose each class (compare integrators, check luminance, visual inspection)
  - Tools and techniques: average luminance, test scenes, debug rendering
  - PT vs BDPT comparison as debugging tool (should produce similar results)
  - How to isolate bugs (simple test scenes, eliminate variables)
  - Step-by-step debugging workflow examples

## BDPT Implementation Guide
- **Requested**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **Requested by**: Agent debugging texture inconsistency bug
- **Reason**: BDPT is complex, no guide exists for understanding or modifying it
- **Current state**: Algorithm documented in integrator-system.md, but no implementation guide
- **Needed information**:
  - Code walkthrough: where path construction happens, where connections happen
  - Vertex structure and what data it stores (why it embeds SurfaceInteraction)
  - Common pitfalls when modifying BDPT (data preservation, PDF calculations)
  - How to verify BDPT changes (must match PT results for simple scenes)
  - Connection strategies breakdown with code references
  - What to test when changing BDPT code

## Material and Texture System Data Flow
- **Requested**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **Requested by**: Agent debugging texture inconsistency bug
- **Reason**: How UV coordinates flow from geometry to material evaluation was unclear
- **Current state**: Individual systems documented but not the data flow between them
- **Needed information**:
  - Complete data flow: Geometry.Hit() → SurfaceInteraction → Material.Scatter/EvaluateBRDF → ColorSource.Evaluate
  - What each component is responsible for (geometry computes UV, material uses it)
  - SurfaceInteraction as the data carrier (what fields it has, why they're all needed)
  - Diagram showing UV coordinate journey through the pipeline
  - Why preserving complete SurfaceInteraction is critical (don't reconstruct partially)
  - Examples of correct data flow in both PT and BDPT

## Common Bug Patterns in Raytracers
- **Requested**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **Requested by**: Agent debugging texture inconsistency bug
- **Reason**: Would help identify and prevent entire classes of bugs
- **Current state**: None
- **Needed information**:
  - Data loss bugs (not preserving UV, normals, etc. through pipeline)
  - Coordinate system bugs (world vs local, UV wrapping)
  - Integrator inconsistencies (PT vs BDPT producing different results)
  - Floating-point precision issues
  - Parallel rendering bugs (race conditions, determinism)
  - How to recognize each pattern and standard fixes
  - Real examples from this codebase (like the UV bug we just fixed)


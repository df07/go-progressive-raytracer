# Testing and Debugging Guides

This directory contains practical guides for testing, debugging, and working with the raytracer.

## Guides

### [CLI Usage and Testing Workflow](cli-usage.md)

Complete reference for CLI flags and testing workflows.

**When to use**:
- Running renders for debugging or comparison
- Setting up profiling
- Comparing PT vs BDPT output
- Understanding output file locations

**Key sections**:
- Complete CLI flag reference
- Output behavior and file naming
- Quick test render recipes (1s, 5s, 30s)
- Typical debugging workflows

### [Testing Strategy](testing-strategy.md)

Comprehensive guide to the three-level testing strategy.

**When to use**:
- Writing new tests for integrators
- Understanding test coverage gaps
- Debugging test failures
- Verifying integrator correctness

**Key sections**:
- Unit tests (component isolation)
- Integration tests (luminance comparison)
- Visual tests (artifact detection)
- When bugs appear at different levels

### [Debugging Rendering Issues](debugging-rendering-issues.md)

Systematic approach to diagnosing rendering bugs.

**When to use**:
- PT and BDPT produce different results
- Investigating brightness differences
- Debugging texture sampling issues
- Tracking down crashes or NaNs

**Key sections**:
- Common bug classes (brightness, color, artifacts, crashes)
- Diagnostic workflows for each bug type
- PT vs BDPT comparison techniques
- Step-by-step debugging examples

### [Common Bug Patterns](common-bug-patterns.md)

Catalog of recurring bug patterns with fixes.

**When to use**:
- Recognizing familiar bug symptoms
- Learning from past mistakes
- Understanding real-world examples
- Quick pattern-matching for debugging

**Key sections**:
- Data loss bugs (UV coordinates, normals)
- Coordinate system bugs
- Integrator inconsistencies
- Floating-point and parallelism issues
- Scale-dependent bugs
- Real examples from this codebase

## Quick Reference

### Debugging Workflow

1. **Classify the bug** - Use [Common Bug Patterns](common-bug-patterns.md) to identify type
2. **Run comparison test** - Use [CLI Usage](cli-usage.md) to compare PT vs BDPT
3. **Check integration tests** - Use [Testing Strategy](testing-strategy.md) to verify
4. **Apply systematic debugging** - Use [Debugging Guide](debugging-rendering-issues.md)

### When to Read Each Guide

**Starting a debugging session**: Start with [Common Bug Patterns](common-bug-patterns.md) to classify

**Need to compare integrators**: Use [CLI Usage](cli-usage.md) for quick recipes

**Writing or fixing tests**: Use [Testing Strategy](testing-strategy.md) for patterns

**Stuck on a specific bug**: Use [Debugging Guide](debugging-rendering-issues.md) for workflows

## Access Log

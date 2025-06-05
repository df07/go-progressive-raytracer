# Progressive Raytracer Development Rules

## Project Process
- All specs go in `/specs` folder in markdown format
- Write specs before implementing major features
- Update rules when completing units of work

## Code Quality
- Always run `go fmt ./...` before checking in
- Always run `go vet ./...` and fix issues before checking in
- Run `go test ./...` to ensure all tests pass

## Testing Guidelines
- Test edge cases and tricky scenarios, not trivial operations
- Use table-driven tests for related test cases
- Focus on mathematical correctness (intersections, normals, bounds)
- Use proper floating-point tolerances (1e-9) for comparisons
- Keep tests clean - remove unnecessary comments and debug code

## Raytracer-Specific
- Use progressive refinement - start simple, add complexity incrementally
- Prioritize correctness over performance initially
- Make rendering results reproducible and testable
- Design for progressive enhancement (multiple passes) 
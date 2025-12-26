---
name: fix-bug
description: Intelligently debug and fix bugs with a repro-first approach. Use when the user reports a bug, unexpected behavior, or asks to fix an issue. Establishes reproducible test cases before fixing.
---

# Bugfix - Intelligent Bug Debugging and Fixing

## When to Use This Skill

Use this skill when:
- The user reports a bug or unexpected behavior
- The user asks to fix an issue or error
- The user says something isn't working correctly
- The user invokes `/bugfix [description]`

## Core Philosophy

**Understand, Reproduce, Fix**: First consult documentation to understand expected behavior, then establish a reproducible test case, and finally fix the bug. If you can't reproduce it, you can't verify the fix.

## Workflow

### Step 1: Read Relevant Documentation

**Use the read-docs skill** to search for and read documentation in `./docs/`:
- Search for docs related to the bug component/system
- Read all relevant documentation files
- Follow read-docs skill procedures: log access, score helpfulness, request missing docs

**Decision Point - Are docs sufficient?**

→ **Docs are SUFFICIENT** (complete, accurate, helpful):
   - You understand the expected behavior
   - You know what the bug is deviating from
   - **Proceed to Step 2**

→ **Docs are INSUFFICIENT** (missing, incomplete, or outdated):
   - You logged access with -1 or +0 score
   - You added documentation request(s) to `./docs/documentation-requests.md`
   - Now evaluate: **Is the missing information CRITICAL to understanding the bug?**

   **CRITICAL means:**
   - You cannot determine expected behavior without it
   - You don't understand how the component should work
   - The bug involves behavior that isn't documented
   - You would be guessing at what "correct" means

   **If CRITICAL → STOP and invoke docs-maintainer:**
   - **DO NOT proceed to Step 2**
   - **DO NOT "examine the code directly"**
   - **DO NOT "work around the gap"**
   - Instead: Use Task tool with `subagent_type=docs-maintainer`
   - Provide context about what documentation is needed
   - Wait for documentation to be created
   - **Return to Step 1** (read the newly created docs)

   **If NOT CRITICAL (only minor details missing):**
   - The overall expected behavior is clear from docs
   - You just need minor implementation details from code
   - **Proceed to Step 2** (use code inspection to fill gaps)

**What documentation tells you**:
- What the correct behavior should be
- What the bug is deviating from
- Where to look for the root cause
- What test cases to write

See `.claude/skills/read-docs/SKILL.md` for complete documentation access procedures.

### Step 2: Establish a Reproducible Test Case

1. **Check for existing failing tests**
   - Run the test suite: `go test ./...`
   - Look for tests related to the bug description
   - If a test is already failing for this issue, use it as the repro

2. **If no failing test exists, create one**
   - Identify the affected package/module
   - Write a minimal test that demonstrates the bug
   - The test should:
     - Be focused and minimal (test one thing)
     - Have a clear assertion that fails due to the bug
     - Include a descriptive name explaining what should happen
   - Run the new test to confirm it fails: `go test -v ./pkg/[package]/ -run TestName`

3. **If you cannot establish a repro**
   - **STOP immediately**
   - Explain to the user what you've tried
   - Ask clarifying questions:
     - What exact steps trigger the bug?
     - What is the expected vs actual behavior?
     - Can they provide example inputs/outputs?
     - Are there specific scenes or parameters that trigger it?
   - Do NOT proceed to debugging without a repro

### Step 3: Debug the Issue

1. **Analyze the failing test**
   - Examine the test output and error messages
   - Identify the root cause (not just symptoms)
   - Use the Read tool to examine relevant source files
   - Look for:
     - Logic errors
     - Edge cases not handled
     - Race conditions (if parallel code)
     - Numerical precision issues (common in raytracers)
     - Incorrect assumptions

2. **Form a hypothesis**
   - State clearly what you believe is causing the bug
   - Explain the reasoning
   - If uncertain, use debugging techniques:
     - Add temporary logging/prints
     - Check intermediate values
     - Verify assumptions with additional test assertions

### Step 4: Implement the Fix

1. **Make targeted changes**
   - Fix the root cause, not symptoms
   - Keep changes minimal and focused
   - Follow the existing code style
   - Avoid over-engineering or adding unnecessary features

2. **Consider edge cases**
   - Does the fix handle boundary conditions?
   - Are there similar bugs elsewhere in the codebase?
   - Does the fix introduce new issues?

### Step 5: Verify the Fix

1. **Run the reproduction test**
   - `go test -v ./pkg/[package]/ -run TestName`
   - The test that was failing must now pass
   - If it still fails, return to Step 3

2. **Run the full test suite**
   - `go test ./...`
   - Ensure no regressions were introduced
   - If any other tests fail, investigate and fix

3. **Manual verification (if applicable)**
   - If the bug affects visual output or interactive features:
     - Test with the CLI: `./raytracer --scene=... --max-samples=10`
     - Or test with the web interface
   - Verify the expected behavior is now correct

### Step 6: Clean Up

1. **Remove debugging code**
   - Remove any temporary logging, prints, or debug statements
   - Keep the test case you created

2. **Document if needed**
   - If the bug was subtle or non-obvious, add a comment explaining why the fix is necessary
   - Update any relevant documentation

## Special Considerations for Raytracer Bugs

**Rendering Issues**:
- Always verify tests with visual output when fixing rendering bugs
- Be careful with floating-point comparisons (use epsilon for tolerance)
- Consider both path-tracing and BDPT integrators

**Performance Issues**:
- Use benchmarks, not just tests: `go test -bench=. ./pkg/[package]/`
- Profile if needed: `--profile=cpu.prof` then `go tool pprof cpu.prof`

**Parallel/Concurrency Bugs**:
- These are hard to reproduce - run tests multiple times
- Look for race conditions: `go test -race ./...`
- Check for proper synchronization in worker pools

**BVH/Acceleration Structure Bugs**:
- Test with scenes that stress the BVH: `spheregrid`, `trianglemesh`, `dragon`
- Verify both correctness and performance

## Examples

### Example 1: Visual Rendering Bug

```
User: "/bugfix The cornell box scene has black artifacts in the corners"

Response:
1. First, let me check documentation about the Cornell box scene and rendering...
   [Uses read-docs skill: searches, reads ./docs/scenes.md and ./docs/rendering.md]
   [Logs access per read-docs procedures]

2. Based on the docs, Cornell box should have uniform lighting. Let me check for
   existing tests...
   [Checks test files]

3. I don't see a failing test. Let me create a test that renders the cornell
   scene and checks for black pixels (RGB values below a threshold in expected
   areas):
   [Creates test in pkg/scene/cornell_test.go]

4. Running the test...
   [Test fails, confirms the bug]

5. Now analyzing the cornell scene setup and rendering code...
   [Investigates light sources, material properties, ray intersection]

6. Found the issue: [explains root cause]
   [Implements fix]

7. Re-running the test...
   [Test passes]

8. Running full test suite...
   [All tests pass]

The bug is fixed! The issue was [explanation].
```

### Example 2: Cannot Reproduce

```
User: "/bugfix Random crashes when rendering"

Response:
1. Let me check documentation about the rendering system and known issues...
   [Uses read-docs skill: reads ./docs/renderer.md]
   [Logs access, scores +0 - found general overview only]

2. Documentation doesn't mention crashes. Let me check for existing tests and
   try to reproduce...
   [Attempts to create a test, runs various scenarios]

3. I cannot establish a reliable reproduction of this crash. I need more
   information:

   - Which scene(s) cause the crash?
   - What command line arguments are you using?
   - Does it happen after a certain number of passes/samples?
   - Are you using CLI or web interface?
   - What does the error message say (if any)?

   Once I can reproduce it, I'll be able to debug and fix it.
```

### Example 3: Existing Failing Test

```
User: "/bugfix Tests are failing in pkg/geometry"

Response:
1. First, let me check documentation about the geometry package and BVH...
   [Uses read-docs skill: reads ./docs/bvh.md]
   [Logs access, scores +1 - found helpful info]

2. Documentation explains BVH should use median-split. Running the test suite
   to identify which test is failing...
   [go test ./pkg/geometry/]

3. Found failing test: TestBVHIntersection
   [Shows test output]

4. This test is our repro. Analyzing why it fails based on expected behavior
   from docs...
   [Investigates the code]

5. Root cause: [explanation]
   [Implements fix]

6. Verifying fix...
   [Test now passes, full suite passes]

Fixed! The issue was [explanation].
```

### Example 4: Missing Documentation (Critical)

```
User: "/bugfix BDPT integrator produces incorrect brightness"

Response:
1. Let me check documentation about BDPT implementation...
   [Uses read-docs skill: reads ./docs/integrators.md]
   [Logs access, scores -1 - missing MIS and brightness details]
   [Adds documentation request per read-docs procedures]

2. The missing documentation on MIS weighting is critical to understanding the
   expected behavior. Let me invoke the docs-maintainer agent to create this
   documentation before proceeding...
   [Uses Task tool with subagent_type=docs-maintainer]
   [Agent creates documentation on BDPT MIS weighting]

3. Now reading the newly created documentation...
   [Reads updated ./docs/integrators.md]
   [Logs access, scores +1 - found needed MIS details]

4. Based on the documentation, I now understand the expected MIS calculation.
   Creating a test that verifies brightness values...
   [Creates test that checks brightness values]
   [Test fails, confirms bug]

5. Analyzing code against documented behavior...
   [Investigates BDPT implementation]

6. Found the issue: MIS weights not properly normalized per the documented formula
   [Implements fix]

7. Verifying fix...
   [Test passes, full suite passes]

Fixed! The bug was in the MIS weight normalization.
```

## Common Pitfalls to Avoid

- **Don't skip documentation** - Always check docs first (follow read-docs skill procedures)
- **DON'T PROCEED TO STEP 2 WHEN DOCS ARE CRITICAL** - This is the most common mistake:
  - If you can't determine expected behavior from docs, STOP
  - If you say "let me examine the code directly" when docs are missing, you're doing it wrong
  - Invoke docs-maintainer first, wait for docs, then proceed
  - Code inspection is NOT a substitute for understanding expected behavior
- **Don't guess at fixes** - Always understand the root cause first
- **Don't skip the repro** - "It seems to work now" is not sufficient
- **Don't introduce regressions** - Always run the full test suite
- **Don't over-complicate** - Simple, targeted fixes are better
- **Don't leave debug code** - Clean up before finishing

## Integration with Project Testing

This raytracer project has:
- Unit tests in each package: `pkg/*/[package]_test.go`
- Test helper utilities
- Benchmark support
- Race detection available

Use these existing tools and patterns when creating reproduction tests.

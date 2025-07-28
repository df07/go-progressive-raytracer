#!/bin/bash

# Simple benchmark script for comparing performance before/after changes
# Usage: ./benchmark.sh

set -e

SCENE="cornell-boxes"
INTEGRATOR="bdpt"
SAMPLES=50
PASSES=1
WORKERS=10
RUNS=2

echo "=== BDPT Performance Benchmark ==="
echo "Scene: $SCENE, Samples: $SAMPLES, Passes: $PASSES, Workers: $WORKERS"
echo "Running $RUNS iterations for each configuration"
echo ""

# Check if there are uncommitted changes (only working directory, ignore staged)
if ! git diff --quiet; then
    echo "Found uncommitted changes to benchmark..."
    
    # Stash changes
    echo "Stashing current changes..."
    git stash push -m "benchmark: temporary stash for performance testing"
    STASHED=true
else
    echo "No uncommitted changes found - running same configuration twice for consistency check."
    STASHED=false
fi

# Build and run BEFORE (baseline)
echo "=== BUILDING BASELINE ==="
go build -o raytracer main.go

echo ""
echo "=== RUNNING BASELINE ($RUNS runs) ==="
BEFORE_TIMES=()
for i in $(seq 1 $RUNS); do
    echo "Baseline run $i/$RUNS..."
    START_TIME=$(date +%s.%N)
    ./raytracer --scene=$SCENE --integrator=$INTEGRATOR --max-samples=$SAMPLES --max-passes=$PASSES --workers=$WORKERS > /dev/null 2>&1
    END_TIME=$(date +%s.%N)
    RUNTIME=$(echo "$END_TIME - $START_TIME" | bc -l)
    BEFORE_TIMES+=($RUNTIME)
    printf "  Time: %.3fs\n" $RUNTIME
done

# Restore changes if we stashed them
if [ "$STASHED" = true ]; then
    echo ""
    echo "=== RESTORING CHANGES ==="
    git stash pop
fi

# Build and run AFTER (with changes)
echo ""
echo "=== BUILDING WITH CHANGES ==="
go build -o raytracer main.go

echo ""
echo "=== RUNNING WITH CHANGES ($RUNS runs) ==="
AFTER_TIMES=()
for i in $(seq 1 $RUNS); do
    echo "Changes run $i/$RUNS..."
    START_TIME=$(date +%s.%N)
    ./raytracer --scene=$SCENE --integrator=$INTEGRATOR --max-samples=$SAMPLES --max-passes=$PASSES --workers=$WORKERS > /dev/null 2>&1
    END_TIME=$(date +%s.%N)
    RUNTIME=$(echo "$END_TIME - $START_TIME" | bc -l)
    AFTER_TIMES+=($RUNTIME)
    printf "  Time: %.3fs\n" $RUNTIME
done

# Calculate averages
echo ""
echo "=== RESULTS ==="

# Calculate average for BEFORE
BEFORE_SUM=0
for time in "${BEFORE_TIMES[@]}"; do
    BEFORE_SUM=$(echo "$BEFORE_SUM + $time" | bc -l)
done
BEFORE_AVG=$(echo "scale=3; $BEFORE_SUM / ${#BEFORE_TIMES[@]}" | bc -l)

# Calculate average for AFTER  
AFTER_SUM=0
for time in "${AFTER_TIMES[@]}"; do
    AFTER_SUM=$(echo "$AFTER_SUM + $time" | bc -l)
done
AFTER_AVG=$(echo "scale=3; $AFTER_SUM / ${#AFTER_TIMES[@]}" | bc -l)

# Calculate improvement
DIFF=$(echo "scale=3; $BEFORE_AVG - $AFTER_AVG" | bc -l)
PERCENT=$(echo "scale=1; ($DIFF / $BEFORE_AVG) * 100" | bc -l)

printf "Baseline average:   %.3fs\n" $BEFORE_AVG
printf "Changes average:    %.3fs\n" $AFTER_AVG
printf "Difference:         %.3fs\n" $DIFF

if (( $(echo "$DIFF > 0" | bc -l) )); then
    printf "Performance:        %.1f%% FASTER\n" $PERCENT
elif (( $(echo "$DIFF < 0" | bc -l) )); then
    PERCENT=$(echo "$PERCENT * -1" | bc -l)
    printf "Performance:        %.1f%% SLOWER\n" $PERCENT
else
    printf "Performance:        NO CHANGE\n"
fi

echo ""
echo "Benchmark complete!"
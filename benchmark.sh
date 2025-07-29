#!/bin/bash

# Simple benchmark script for comparing performance before/after changes
# Usage: ./benchmark.sh

set -e

SCENE="cornell-boxes"
INTEGRATOR="bdpt"
SAMPLES=10
PASSES=1
WORKERS=10
RUNS=10

echo "=== BDPT Performance Benchmark ==="
echo "Scene: $SCENE, Samples: $SAMPLES, Passes: $PASSES, Workers: $WORKERS"
echo "Running $RUNS iterations for each configuration"
echo ""

# Check if there are uncommitted changes (only working directory, ignore staged and benchmark.sh itself)
if ! git diff --quiet -- ':!benchmark.sh'; then
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
    printf "Baseline run %2d/%d... " $i $RUNS
    START_TIME=$(date +%s.%N)
    ./raytracer --scene=$SCENE --integrator=$INTEGRATOR --max-samples=$SAMPLES --max-passes=$PASSES --workers=$WORKERS > /dev/null 2>&1
    END_TIME=$(date +%s.%N)
    RUNTIME=$(echo "$END_TIME - $START_TIME" | bc -l)
    BEFORE_TIMES+=($RUNTIME)
    printf "%6.3fs\n" $RUNTIME
done

# Restore changes if we stashed them
if [ "$STASHED" = true ]; then
    echo ""
    echo "=== RESTORING CHANGES ==="
    git stash pop
fi

# Cooling period between baseline and changes
echo ""
echo "=== COOLING PERIOD ==="
echo "Waiting 3 seconds to reduce thermal effects..."
sleep 3

# Build and run AFTER (with changes)
echo ""
echo "=== BUILDING WITH CHANGES ==="
go build -o raytracer main.go

echo ""
echo "=== RUNNING WITH CHANGES ($RUNS runs) ==="
AFTER_TIMES=()
for i in $(seq 1 $RUNS); do
    printf "Changes run %2d/%d...  " $i $RUNS
    START_TIME=$(date +%s.%N)
    ./raytracer --scene=$SCENE --integrator=$INTEGRATOR --max-samples=$SAMPLES --max-passes=$PASSES --workers=$WORKERS > /dev/null 2>&1
    END_TIME=$(date +%s.%N)
    RUNTIME=$(echo "$END_TIME - $START_TIME" | bc -l)
    AFTER_TIMES+=($RUNTIME)
    printf "%6.3fs\n" $RUNTIME
done

# Calculate averages
echo ""
echo "=== RESULTS ==="

# Sort arrays and calculate trimmed means (drop highest and lowest)
BEFORE_SORTED=($(printf '%s\n' "${BEFORE_TIMES[@]}" | sort -n))
AFTER_SORTED=($(printf '%s\n' "${AFTER_TIMES[@]}" | sort -n))

# Calculate trimmed mean for BEFORE (drop first and last)
BEFORE_SUM=0
TRIM_COUNT=$((${#BEFORE_SORTED[@]} - 2))
for i in $(seq 1 $TRIM_COUNT); do
    BEFORE_SUM=$(echo "$BEFORE_SUM + ${BEFORE_SORTED[$i]}" | bc -l)
done
BEFORE_AVG=$(echo "scale=3; $BEFORE_SUM / $TRIM_COUNT" | bc -l)

# Calculate trimmed mean for AFTER (drop first and last)
AFTER_SUM=0
for i in $(seq 1 $TRIM_COUNT); do
    AFTER_SUM=$(echo "$AFTER_SUM + ${AFTER_SORTED[$i]}" | bc -l)
done
AFTER_AVG=$(echo "scale=3; $AFTER_SUM / $TRIM_COUNT" | bc -l)

# Calculate improvement
DIFF=$(echo "scale=3; $BEFORE_AVG - $AFTER_AVG" | bc -l)
PERCENT=$(echo "scale=2; ($DIFF / $BEFORE_AVG) * 100" | bc -l)

printf "Baseline trimmed:   %.3fs (dropped %.3fs, %.3fs)\n" $BEFORE_AVG ${BEFORE_SORTED[0]} ${BEFORE_SORTED[-1]}
printf "Changes trimmed:    %.3fs (dropped %.3fs, %.3fs)\n" $AFTER_AVG ${AFTER_SORTED[0]} ${AFTER_SORTED[-1]}
printf "Difference:         %.3fs\n" $DIFF

if (( $(echo "$DIFF > 0" | bc -l) )); then
    printf "Performance:        %.2f%% FASTER\n" $PERCENT
elif (( $(echo "$DIFF < 0" | bc -l) )); then
    PERCENT_ABS=$(echo "$PERCENT * -1" | bc -l)
    printf "Performance:        %.2f%% SLOWER\n" $PERCENT_ABS
else
    printf "Performance:        NO CHANGE\n"
fi

echo ""
echo "Benchmark complete!"
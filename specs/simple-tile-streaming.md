# Simple Real-Time Tile Streaming

## Goal
Stream individual tiles to the web frontend as they complete, instead of waiting for entire passes to finish.

## Current Problem
- Web interface only updates after ALL tiles in a pass complete (several seconds delay)
- User sees no progress for long periods
- Each update sends entire ~500KB image even if only one tile changed

## Simple Solution
Add tile completion callback to existing worker pool + WebSocket streaming of individual tiles.

## Minimal Changes Required

### 1. Add Tile Completion Callback to Worker Pool
```go
// In pkg/renderer/worker_pool.go
type WorkerPool struct {
    // ... existing fields
    onTileComplete func(x, y int, tileImage *image.RGBA) // NEW
}

// In worker.go
func (w *Worker) processTask(task *TileTask) {
    // ... existing tile rendering logic
    
    // NEW: Extract just this tile's pixels and call callback
    if w.pool.onTileComplete != nil {
        tileImage := extractTileImage(task, w.pool.pixelStats)
        w.pool.onTileComplete(task.StartX/64, task.StartY/64, tileImage)
    }
}
```

### 2. WebSocket Tile Updates
```go
// In web/server/server.go
func (s *Server) handleStreamRender(w http.ResponseWriter, r *http.Request) {
    // Upgrade to WebSocket
    conn, _ := upgrader.Upgrade(w, r, nil)
    
    // Create renderer with tile callback
    raytracer := renderer.NewProgressiveRaytracer(scene, width, height)
    raytracer.WorkerPool.SetTileCallback(func(x, y int, tileImage *image.RGBA) {
        // Send tile update via WebSocket
        update := TileUpdate{
            TileX: x,
            TileY: y, 
            ImageData: imageToBase64PNG(tileImage),
        }
        conn.WriteJSON(update)
    })
    
    // Start rendering (existing code)
    raytracer.Render()
}

type TileUpdate struct {
    TileX     int    `json:"tileX"`
    TileY     int    `json:"tileY"`  
    ImageData string `json:"imageData"`
}
```

### 3. Client-Side Tile Compositor
```javascript
// In web/static/js (new file: tile-compositor.js)
class TileCompositor {
    constructor(canvas) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.tileSize = 64;
    }
    
    updateTile(tileX, tileY, imageDataUrl) {
        const img = new Image();
        img.onload = () => {
            const x = tileX * this.tileSize;
            const y = tileY * this.tileSize;
            this.ctx.drawImage(img, x, y);
        };
        img.src = imageDataUrl;
    }
}

// Update existing WebSocket handler
ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.tileX !== undefined) {
        // Tile update
        tileCompositor.updateTile(data.tileX, data.tileY, data.imageData);
    } else {
        // Full image update (keep existing logic as fallback)
        updateFullImage(data.imageData);
    }
};
```

## CLI Behavior (Unchanged)
- CLI mode: Don't set tile callback, works exactly as before
- Same file output, same performance, same interface

## Implementation Plan

### Phase 1: Basic Tile Streaming
1. Add `onTileComplete` callback to `WorkerPool`
2. Add `extractTileImage()` helper function
3. Create WebSocket endpoint that sets the callback
4. Simple JavaScript tile compositor
5. Test that CLI behavior is unchanged

### Phase 2: Optimizations
1. Send smaller tile updates (PNG compress individual tiles)
2. Add basic error handling and reconnection
3. Optional: Send tile coordinates + raw pixel data instead of base64 PNG

## Testing
- Verify CLI rendering unchanged
- Test tile updates appear in correct positions
- Verify final image matches traditional rendering
- Test WebSocket connection handling

## Benefits
- ~100 tile updates instead of ~10 pass updates
- Immediate visual feedback as tiles complete
- Minimal code changes to existing clean architecture
- No complex event systems or dual-mode renderers

This is a much simpler approach that achieves the core goal without architectural complexity.
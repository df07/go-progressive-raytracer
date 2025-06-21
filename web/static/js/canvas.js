class RenderCanvas {
    constructor(canvas, tileSize = 64) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.tileSize = tileSize;
        this.tileGrid = new Map(); // TileID -> ImageData
        
        // Track canvas dimensions for proper scaling
        this.imageWidth = 0;
        this.imageHeight = 0;
        
        // Error tracking for monitoring
        this.errorCount = 0;
        this.totalTileUpdates = 0;
        
        // Animation system for fade-in effects
        this.animatingTiles = new Map(); // TileID -> {image, x, y, startTime, opacity, renderID}
        this.animationDuration = 3000; // 3000ms fade-in
        this.animationRunning = false;
        this.currentRenderID = 0; // Used to invalidate old animations
    }
    
    // Initialize canvas for a new render
    initCanvas(width, height) {
        try {
            // Increment render ID to invalidate any ongoing animations FIRST
            this.currentRenderID++;
            
            // Stop any running animations immediately
            this.animationRunning = false;
            
            // Clear previous tiles and animation state immediately
            this.tileGrid.clear();
            this.animatingTiles.clear();
            
            this.imageWidth = width;
            this.imageHeight = height;
            
            // Set canvas size (this also clears the canvas)
            this.canvas.width = width;
            this.canvas.height = height;
            
            // Reset counters
            this.errorCount = 0;
            this.totalTileUpdates = 0;
            
            // Explicitly clear canvas with background color
            this.ctx.fillStyle = '#1a1a1a'; // Dark background
            this.ctx.fillRect(0, 0, width, height);
        } catch (error) {
            console.error('Error initializing canvas:', error);
            this.errorCount++;
        }
    }
    
    // Update a single tile
    updateTile(tileX, tileY, imageDataUrl) {
        try {
            this.totalTileUpdates++;
            
            const img = new Image();
            img.onload = () => {
                try {
                    const pixelX = tileX * this.tileSize;
                    const pixelY = tileY * this.tileSize;
                    
                    // Store tile for future reference
                    const tileID = `${tileX}_${tileY}`;
                    this.tileGrid.set(tileID, img);
                    
                    // Start fade-in animation for this tile
                    this.startTileAnimation(tileID, img, pixelX, pixelY);
                } catch (error) {
                    this.handleTileError(tileX, tileY, 'Error processing tile image', error);
                }
            };
            
            img.onerror = () => {
                this.handleTileError(tileX, tileY, 'Failed to load tile image');
            };
            
            img.src = imageDataUrl;
        } catch (error) {
            this.handleTileError(tileX, tileY, 'Error updating tile', error);
        }
    }
    
    // Start fade-in animation for a tile
    startTileAnimation(tileID, image, x, y) {
        const now = performance.now();
        const renderID = this.currentRenderID;
        
        // If this tile is already animating, remove the old animation
        if (this.animatingTiles.has(tileID)) {
            this.animatingTiles.delete(tileID);
        }
        
        this.animatingTiles.set(tileID, {
            image: image,
            x: x,
            y: y,
            startTime: now,
            opacity: 0,
            renderID: renderID // Tag with current render ID
        });
        
        // Start animation loop if not already running
        if (!this.animationRunning) {
            this.animationRunning = true;
            this.animationLoop(renderID);
        }
    }
    
    // Animation loop for fade-in effects
    animationLoop(renderID) {
        // Exit immediately if this animation loop is from an old render
        if (renderID !== this.currentRenderID) {
            return;
        }
        
        const now = performance.now();
        const ctx = this.ctx;
        
        // Draw animating tiles with their current opacity
        let hasActiveAnimations = false;
        
        for (const [tileID, tileData] of this.animatingTiles) {
            // Skip tiles from old renders
            if (tileData.renderID !== this.currentRenderID) {
                this.animatingTiles.delete(tileID);
                continue;
            }
            
            const elapsed = now - tileData.startTime;
            const progress = Math.min(elapsed / this.animationDuration, 1.0);
            
            // Smooth easing function (ease-out)
            const easedProgress = 1 - Math.pow(1 - progress, 3);
            
            // Only redraw if opacity has changed significantly (avoid excessive redraws)
            const newOpacity = easedProgress;
            if (Math.abs(newOpacity - tileData.opacity) > 0.02 || progress >= 1.0) {
                tileData.opacity = newOpacity;
                
                // Draw the tile with current opacity
                ctx.save();
                ctx.globalAlpha = tileData.opacity;
                ctx.drawImage(tileData.image, tileData.x, tileData.y);
                ctx.restore();
            }
            
            // Remove completed animations
            if (progress >= 1.0) {
                this.animatingTiles.delete(tileID);
            } else {
                hasActiveAnimations = true;
            }
        }
        
        // Continue animation loop if there are active animations and this is still the current render
        if (hasActiveAnimations && renderID === this.currentRenderID) {
            requestAnimationFrame(() => this.animationLoop(renderID));
        } else {
            this.animationRunning = false;
        }
    }
    

    
    // Get canvas as data URL for saving/display
    getDataURL() {
        return this.canvas.toDataURL('image/png');
    }
    
    // Handle canvas click for pixel inspection
    handleClick(event, callback) {
        if (!callback) return;
        
        const rect = this.canvas.getBoundingClientRect();
        const scaleX = this.canvas.width / rect.width;
        const scaleY = this.canvas.height / rect.height;
        
        const x = Math.floor((event.clientX - rect.left) * scaleX);
        const y = Math.floor((event.clientY - rect.top) * scaleY);
        
        callback(x, y);
    }
    
    // Clear the canvas
    clear() {
        // Increment render ID to invalidate any ongoing animations
        this.currentRenderID++;
        
        // Stop any running animations immediately
        this.animationRunning = false;
        
        this.ctx.fillStyle = '#1a1a1a';
        this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
        this.tileGrid.clear();
        this.animatingTiles.clear();
    }
    
    // Handle tile errors with consistent logging and metrics
    handleTileError(tileX, tileY, message, error = null) {
        this.errorCount++;
        const errorMsg = `Tile (${tileX}, ${tileY}): ${message}`;
        
        if (error) {
            console.error(errorMsg, error);
        } else {
            console.warn(errorMsg);
        }
        
        // Log periodic error rate for monitoring
        if (this.errorCount % 10 === 1 && this.totalTileUpdates > 0) {
            const errorRate = (this.errorCount / this.totalTileUpdates * 100).toFixed(1);
            console.warn(`Tile error rate: ${this.errorCount}/${this.totalTileUpdates} (${errorRate}%)`);
        }
    }
    
    // Get error statistics for monitoring
    getErrorStats() {
        const successRate = this.totalTileUpdates > 0 ? 
            ((this.totalTileUpdates - this.errorCount) / this.totalTileUpdates * 100).toFixed(1) : 
            '100.0';
        
        return {
            totalTileUpdates: this.totalTileUpdates,
            errorCount: this.errorCount,
            successRate: parseFloat(successRate),
            activeAnimations: this.animatingTiles.size
        };
    }
    
    // Set animation duration (in milliseconds)
    setAnimationDuration(duration) {
        this.animationDuration = Math.max(50, Math.min(5000, duration)); // Clamp between 50ms and 5000ms
    }
    
    // Get current animation status
    getAnimationStatus() {
        return {
            isAnimating: this.animationRunning,
            activeAnimations: this.animatingTiles.size,
            animationDuration: this.animationDuration
        };
    }
    
    // Force stop all animations immediately (call when stopping render)
    stopAllAnimations() {
        this.currentRenderID++;
        this.animationRunning = false;
        this.animatingTiles.clear();
    }
}
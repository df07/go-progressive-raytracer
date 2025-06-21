class RenderCanvas {
    constructor(canvas, tileSize = 64) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.tileSize = tileSize;
        this.tileGrid = new Map(); // TileID -> ImageData
        this.renderQueue = [];
        this.renderScheduled = false;
        
        // Track canvas dimensions for proper scaling
        this.imageWidth = 0;
        this.imageHeight = 0;
        
        // Error tracking for monitoring
        this.errorCount = 0;
        this.totalTileUpdates = 0;
    }
    
    // Initialize canvas for a new render
    initCanvas(width, height) {
        try {
            this.imageWidth = width;
            this.imageHeight = height;
            
            // Set canvas size
            this.canvas.width = width;
            this.canvas.height = height;
            
            // Clear previous tiles and reset counters
            this.tileGrid.clear();
            this.renderQueue = [];
            this.errorCount = 0;
            this.totalTileUpdates = 0;
            
            // Clear canvas
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
                    
                    // Queue for rendering (batched for performance)
                    this.queueTileRender(pixelX, pixelY, img);
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
    
    // Queue tile for batched rendering
    queueTileRender(x, y, image) {
        this.renderQueue.push({x, y, image});
        
        if (!this.renderScheduled) {
            this.renderScheduled = true;
            requestAnimationFrame(() => this.processRenderQueue());
        }
    }
    
    // Process all queued tile renders in single frame
    processRenderQueue() {
        try {
            const ctx = this.ctx;
            
            // Process all queued tile updates in single frame
            for (const {x, y, image} of this.renderQueue) {
                try {
                    ctx.drawImage(image, x, y);
                } catch (error) {
                    console.warn('Error drawing tile at (' + x + ', ' + y + '):', error);
                    this.errorCount++;
                }
            }
            
            this.renderQueue.length = 0;
            this.renderScheduled = false;
        } catch (error) {
            console.error('Error processing render queue:', error);
            this.errorCount++;
            this.renderQueue.length = 0;
            this.renderScheduled = false;
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
        this.ctx.fillStyle = '#1a1a1a';
        this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
        this.tileGrid.clear();
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
            successRate: parseFloat(successRate)
        };
    }
}
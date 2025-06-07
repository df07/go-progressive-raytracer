# Progressive Raytracer Web API

A web application for streaming progressive raytracing results in real-time using Server-Sent Events (SSE).

## Features

- **Progressive Rendering**: Watch your images improve with each pass
- **Real-time Streaming**: Images update automatically as they render
- **Interactive Controls**: Adjust resolution, samples, and scene parameters
- **Multiple Scenes**: Cornell Box and Basic Scene support
- **Live Statistics**: Monitor rendering progress and performance

## Quick Start

1. **Build and run the web server:**
   ```bash
   go build -o web-server.exe web/main.go
   web-server.exe -port 8080
   ```

2. **Open your browser:**
   Navigate to `http://localhost:8080`

3. **Start rendering:**
   - Adjust parameters (scene, resolution, samples)
   - Click "Start Rendering"
   - Watch the progressive updates in real-time!

## API Endpoints

### Health Check
```
GET /api/health
```
Returns: `{"status":"ok"}`

### Progressive Rendering (SSE)
```
GET /api/render?scene=cornell-box&width=400&height=400&maxSamples=50&maxPasses=7
```

**Parameters:**
- `scene`: "cornell-box" or "basic" (default: "cornell-box")
- `width`: Image width, 100-2000 (default: 400)
- `height`: Image height, 100-2000 (default: 400)
- `maxSamples`: Maximum samples per pixel, 1-1000 (default: 50)
- `maxPasses`: Maximum number of passes, 1-20 (default: 7)

**Server-Sent Events:**
- `progress`: Contains image data and statistics for each pass
- `complete`: Rendering finished
- `error`: Error occurred

## Architecture

### Backend (Go)
- **Server-Sent Events**: Streams progressive updates
- **Parallel Processing**: Multi-threaded tile-based rendering
- **PNG Encoding**: Images encoded as base64 PNG data
- **Statistics**: Real-time performance metrics

### Frontend (HTML/JavaScript)
- **EventSource API**: Receives SSE streams
- **Dynamic UI**: Updates image and stats in real-time
- **Responsive Design**: Works on desktop and mobile
- **Interactive Controls**: Parameter adjustment

## Example Usage

### Basic Cornell Box (Fast)
```
http://localhost:8080/api/render?scene=cornell-box&width=200&height=200&maxSamples=10&maxPasses=3
```

### High Quality Render
```
http://localhost:8080/api/render?scene=basic&width=800&height=600&maxSamples=100&maxPasses=10
```

## Development

### Adding New Scenes
1. Create scene in `pkg/scene/`
2. Add case to `createScene()` in `pkg/web/server.go`
3. Update frontend scene dropdown

### Customizing Parameters
- Modify validation ranges in `parseRenderRequest()`
- Update HTML form controls and validation
- Adjust default values in `DefaultProgressiveConfig()`

## Technical Details

- **Streaming Protocol**: Server-Sent Events (SSE)
- **Image Format**: PNG encoded as base64
- **Concurrency**: Configurable worker pool
- **Memory**: Shared pixel statistics for efficiency
- **Progressive Algorithm**: Exponential sample increase per pass

## Performance Tips

- **Start Small**: Use 200x200 for testing, scale up for production
- **Balanced Passes**: 5-7 passes usually optimal
- **Monitor CPU**: Adjust worker count based on available cores
- **Network**: Large images may take time to stream over slower connections 
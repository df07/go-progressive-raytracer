class ProgressiveRaytracer {
  constructor() {
      this.eventSource = null;
      this.isRendering = false;
      this.limits = null; // Store server-provided limits
      this.initializeTheme();
      this.bindEvents();
      this.initializeSections();
      this.loadSceneDefaults(); // Load initial defaults
  }

  initializeTheme() {
      // Load saved theme or default to light
      const savedTheme = localStorage.getItem('raytracer-theme') || 'light';
      this.setTheme(savedTheme);
  }

  setTheme(theme) {
      // Update document theme
      document.documentElement.setAttribute('data-theme', theme);
      
      // Update theme toggle buttons
      document.querySelectorAll('.theme-toggle').forEach(toggle => {
          if (toggle.dataset.theme === theme) {
              toggle.classList.add('active');
          } else {
              toggle.classList.remove('active');
          }
      });
      
      // Save theme preference
      localStorage.setItem('raytracer-theme', theme);
  }

  bindEvents() {
      document.getElementById('startBtn').addEventListener('click', () => this.startRendering());
      document.getElementById('stopBtn').addEventListener('click', () => this.stopRendering());
      document.getElementById('scene').addEventListener('change', () => this.loadSceneDefaults());
      
      // Canvas click handler is set up in initializeTileStreaming
      
      // Add section toggle handlers
      document.querySelectorAll('.section-header').forEach(header => {
          header.addEventListener('click', () => this.toggleSection(header));
      });

      // Add theme toggle handlers
      document.querySelectorAll('.theme-toggle').forEach(toggle => {
          toggle.addEventListener('click', () => {
              this.setTheme(toggle.dataset.theme);
          });
      });
  }

  initializeSections() {
      // Initialize collapsible sections
      document.querySelectorAll('.section-header').forEach(header => {
          const isActive = header.classList.contains('active');
          const sectionName = header.dataset.section;
          const content = document.getElementById(`${sectionName}-content`);
          
          if (!isActive && content) {
              content.classList.remove('expanded');
          }
      });
  }

  toggleSection(header) {
      const sectionName = header.dataset.section;
      const content = document.getElementById(`${sectionName}-content`);
      const isExpanded = content.classList.contains('expanded');
      
      if (isExpanded) {
          header.classList.remove('active');
          content.classList.remove('expanded');
      } else {
          header.classList.add('active');
          content.classList.add('expanded');
      }
  }

  async loadSceneDefaults() {
      const scene = document.getElementById('scene').value;
      try {
          const response = await fetch(`/api/scene-config?scene=${scene}`);
          if (response.ok) {
              const config = await response.json();
              
              // Update form controls with scene defaults
              document.getElementById('width').value = config.defaults.width;
              document.getElementById('height').value = config.defaults.height;
              document.getElementById('maxSamples').value = config.defaults.samplesPerPixel;
              document.getElementById('maxPasses').value = config.defaults.maxPasses;
              document.getElementById('rrMinBounces').value = config.defaults.russianRouletteMinBounces;
              document.getElementById('rrMinSamples').value = config.defaults.russianRouletteMinSamples;
              document.getElementById('adaptiveMinSamples').value = config.defaults.adaptiveMinSamples;
              document.getElementById('adaptiveThreshold').value = config.defaults.adaptiveThreshold;
              
              // Apply validation limits from server
              this.applyLimits(config.limits);
              
              // Update scene-specific options
              this.updateSceneOptions(config.sceneOptions || {});
              
              console.log(`Loaded defaults for ${scene}:`, config);
          } else {
              console.error('Failed to load scene config:', response.statusText);
          }
      } catch (error) {
          console.error('Error loading scene config:', error);
      }
  }

  applyLimits(limits) {
      this.limits = limits; // Store limits for validation
      
      // Apply limits to all form controls
      const controls = [
          { id: 'width', limits: limits.width },
          { id: 'height', limits: limits.height },
          { id: 'maxSamples', limits: limits.maxSamples },
          { id: 'maxPasses', limits: limits.maxPasses },
          { id: 'rrMinBounces', limits: limits.russianRouletteMinBounces },
          { id: 'rrMinSamples', limits: limits.russianRouletteMinSamples },
          { id: 'adaptiveMinSamples', limits: limits.adaptiveMinSamples },
          { id: 'adaptiveThreshold', limits: limits.adaptiveThreshold },
      ];

      controls.forEach(control => {
          const element = document.getElementById(control.id);
          if (element && control.limits) {
              element.min = control.limits.min;
              element.max = control.limits.max;
          }
      });
  }

  updateSceneOptions(sceneOptions) {
      const container = document.getElementById('sceneOptions');
      container.innerHTML = ''; // Clear existing options
      
      Object.keys(sceneOptions).forEach(optionKey => {
          const option = sceneOptions[optionKey];
          const controlGroup = document.createElement('div');
          controlGroup.className = 'control-group';
          
          const label = document.createElement('label');
          label.setAttribute('for', optionKey);
          label.textContent = this.formatLabel(optionKey) + ':';
          controlGroup.appendChild(label);
          
          if (option.type === 'select') {
              const select = document.createElement('select');
              select.id = optionKey;
              
              option.options.forEach(optionValue => {
                  const optionElement = document.createElement('option');
                  optionElement.value = optionValue;
                  optionElement.textContent = this.formatOptionText(optionValue);
                  if (optionValue === option.default) {
                      optionElement.selected = true;
                  }
                  select.appendChild(optionElement);
              });
              
              controlGroup.appendChild(select);
          } else if (option.type === 'number') {
              const input = document.createElement('input');
              input.type = 'number';
              input.id = optionKey;
              input.value = option.default;
              input.min = option.min;
              input.max = option.max;
              input.step = 1;
              
              controlGroup.appendChild(input);
              
              // Add tooltip to label
              label.className = 'tooltip';
              label.setAttribute('data-tooltip', `Range: ${option.min} - ${option.max}`);
          }
          
          container.appendChild(controlGroup);
      });
  }

  formatLabel(key) {
      // Convert camelCase to readable labels
      switch (key) {
          case 'cornellGeometry': return 'Cornell Geometry';
          case 'sphereGridSize': return 'Grid Size';
          case 'materialFinish': return 'Material Finish';
          case 'dragonMaterialFinish': return 'Dragon Material';
          default: return key.charAt(0).toUpperCase() + key.slice(1);
      }
  }

  formatOptionText(value) {
      // Format option values for display
      switch (value) {
          case 'spheres': return 'Spheres';
          case 'boxes': return 'Boxes';
          case 'empty': return 'Empty';
          case 'metallic': return 'Metallic';
          case 'matte': return 'Matte';
          case 'glossy': return 'Glossy';
          case 'mirror': return 'Mirror';
          case 'glass': return 'Glass';
          case 'mixed': return 'Mixed';
          case 'gold': return 'Gold';
          case 'plastic': return 'Plastic';
          case 'copper': return 'Copper';
          default: return value.charAt(0).toUpperCase() + value.slice(1);
      }
  }

  buildUrlWithSceneParams(baseUrl, params) {
      // Build URL with scene-specific parameters
      let url = baseUrl;
      
      // Add scene-specific parameters if they exist
      if (params.cornellGeometry) {
          url += url.includes('?') ? '&' : '?';
          url += `cornellGeometry=${params.cornellGeometry}`;
      }
      if (params.sphereGridSize) {
          url += url.includes('?') ? '&' : '?';
          url += `sphereGridSize=${params.sphereGridSize}`;
      }
      if (params.materialFinish) {
          url += url.includes('?') ? '&' : '?';
          url += `materialFinish=${params.materialFinish}`;
      }
      if (params.dragonMaterialFinish) {
          url += url.includes('?') ? '&' : '?';
          url += `dragonMaterialFinish=${params.dragonMaterialFinish}`;
      }
      
      return url;
  }

  startRendering() {
      if (this.isRendering) return;

      // Validate parameters before starting
      const validation = this.validateParameters();
      if (!validation.isValid) {
          this.setStatus('error', validation.error);
          return;
      }

      const params = this.getParameters();
      const url = `/api/render?${new URLSearchParams(params).toString()}`;

      this.setStatus('rendering', 'Starting render...');
      this.isRendering = true;
      this.updateButtons();
      this.resetStats();
      
      // Initialize tile streaming
      this.initializeTileStreaming(params);

      this.eventSource = new EventSource(url);

      this.eventSource.onopen = () => {
          console.log('SSE connection opened');
          this.setStatus('rendering', 'Rendering...');
      };

      // Tile streaming event handler
      this.eventSource.addEventListener('tile', (event) => {
          const data = JSON.parse(event.data);
          this.updateTile(data);
      });

      // Pass completion handler
      this.eventSource.addEventListener('passComplete', (event) => {
          const data = JSON.parse(event.data);
          this.updatePassComplete(data);
      });

      this.eventSource.addEventListener('complete', (event) => {
          console.log('Rendering completed');
          this.setStatus('complete', 'Rendering completed!');
          this.stopRendering();
      });

      this.eventSource.addEventListener('error', (event) => {
          console.error('SSE error:', event.data);
          this.setStatus('error', `Error: ${event.data}`);
          this.stopRendering();
      });

      this.eventSource.onerror = (event) => {
          console.error('SSE connection error:', event);
          // Only show generic connection error if we haven't already shown a specific error
          const statusElement = document.getElementById('status');
          if (!statusElement.classList.contains('error')) {
              this.setStatus('error', 'Connection lost - please check your parameters and try again');
          }
          this.stopRendering();
      };
  }

  stopRendering() {
      if (this.eventSource) {
          this.eventSource.close();
          this.eventSource = null;
      }
      
      // Force stop all canvas animations immediately
      if (this.renderCanvas) {
          this.renderCanvas.stopAllAnimations();
      }
      
      this.isRendering = false;
      this.updateButtons();
      
      if (document.getElementById('status').classList.contains('rendering')) {
          this.setStatus('idle', 'Stopped');
      }
  }

  getParameters() {
      const params = {
          scene: document.getElementById('scene').value,
          width: document.getElementById('width').value,
          height: document.getElementById('height').value,
          maxSamples: document.getElementById('maxSamples').value,
          maxPasses: document.getElementById('maxPasses').value,
          rrMinBounces: document.getElementById('rrMinBounces').value,
          rrMinSamples: document.getElementById('rrMinSamples').value,
          adaptiveMinSamples: document.getElementById('adaptiveMinSamples').value,
          adaptiveThreshold: document.getElementById('adaptiveThreshold').value
      };
      
      // Add scene-specific parameters
      const cornellGeometry = document.getElementById('cornellGeometry');
      if (cornellGeometry) {
          params.cornellGeometry = cornellGeometry.value;
      }
      
      const sphereGridSize = document.getElementById('sphereGridSize');
      if (sphereGridSize) {
          params.sphereGridSize = sphereGridSize.value;
      }
      
      const materialFinish = document.getElementById('materialFinish');
      if (materialFinish) {
          params.materialFinish = materialFinish.value;
      }
      
      const sphereComplexity = document.getElementById('sphereComplexity');
      if (sphereComplexity) {
          params.sphereComplexity = sphereComplexity.value;
      }
      
      const dragonMaterialFinish = document.getElementById('dragonMaterialFinish');
      if (dragonMaterialFinish) {
          params.dragonMaterialFinish = dragonMaterialFinish.value;
      }
      
      return params;
  }

  validateParameters() {
      // Use server-provided limits if available, otherwise fall back to defaults
      const limits = this.limits || {
          width: { min: 100, max: 2000 },
          height: { min: 100, max: 2000 },
          maxSamples: { min: 1, max: 10000 },
          maxPasses: { min: 1, max: 100 },
          russianRouletteMinBounces: { min: 1, max: 20 },
          russianRouletteMinSamples: { min: 1, max: 50 },
          adaptiveMinSamples: { min: 0.01, max: 1.0 },
          adaptiveThreshold: { min: 0.001, max: 0.5 },
      };

      const width = parseInt(document.getElementById('width').value);
      const height = parseInt(document.getElementById('height').value);
      const maxSamples = parseInt(document.getElementById('maxSamples').value);
      const maxPasses = parseInt(document.getElementById('maxPasses').value);
      const rrMinBounces = parseInt(document.getElementById('rrMinBounces').value);
      const rrMinSamples = parseInt(document.getElementById('rrMinSamples').value);
      const adaptiveMinSamples = parseFloat(document.getElementById('adaptiveMinSamples').value);
      const adaptiveThreshold = parseFloat(document.getElementById('adaptiveThreshold').value);

      if (isNaN(width) || width < limits.width.min || width > limits.width.max) {
          return { isValid: false, error: `Width must be between ${limits.width.min} and ${limits.width.max}` };
      }
      if (isNaN(height) || height < limits.height.min || height > limits.height.max) {
          return { isValid: false, error: `Height must be between ${limits.height.min} and ${limits.height.max}` };
      }
      if (isNaN(maxSamples) || maxSamples < limits.maxSamples.min || maxSamples > limits.maxSamples.max) {
          return { isValid: false, error: `Max samples must be between ${limits.maxSamples.min} and ${limits.maxSamples.max}` };
      }
      if (isNaN(maxPasses) || maxPasses < limits.maxPasses.min || maxPasses > limits.maxPasses.max) {
          return { isValid: false, error: `Max passes must be between ${limits.maxPasses.min} and ${limits.maxPasses.max}` };
      }
      if (isNaN(rrMinBounces) || rrMinBounces < limits.russianRouletteMinBounces.min || rrMinBounces > limits.russianRouletteMinBounces.max) {
          return { isValid: false, error: `RR Min Bounces must be between ${limits.russianRouletteMinBounces.min} and ${limits.russianRouletteMinBounces.max}` };
      }
      if (isNaN(rrMinSamples) || rrMinSamples < limits.russianRouletteMinSamples.min || rrMinSamples > limits.russianRouletteMinSamples.max) {
          return { isValid: false, error: `RR Min Samples must be between ${limits.russianRouletteMinSamples.min} and ${limits.russianRouletteMinSamples.max}` };
      }
      if (isNaN(adaptiveMinSamples) || adaptiveMinSamples < limits.adaptiveMinSamples.min || adaptiveMinSamples > limits.adaptiveMinSamples.max) {
          return { isValid: false, error: `Adaptive Min Samples must be between ${limits.adaptiveMinSamples.min} and ${limits.adaptiveMinSamples.max} (percentage of max samples)` };
      }
      if (isNaN(adaptiveThreshold) || adaptiveThreshold < limits.adaptiveThreshold.min || adaptiveThreshold > limits.adaptiveThreshold.max) {
          return { isValid: false, error: `Adaptive Threshold must be between ${limits.adaptiveThreshold.min} and ${limits.adaptiveThreshold.max}` };
      }

      // Performance warnings
      if (width * height > 800 * 600 && maxSamples > 10000) {
          return { isValid: false, error: 'Large images with high samples may be very slow. Try reducing resolution or samples.' };
      }

      return { isValid: true };
  }


  resetStats() {
      document.getElementById('progressFill').style.width = '0%';
      document.getElementById('currentPass').textContent = '-';
      document.getElementById('totalPasses').textContent = '-';
      document.getElementById('avgSamples').textContent = '-';
      document.getElementById('totalSamples').textContent = '-';
      document.getElementById('totalPixels').textContent = '-';
      document.getElementById('primitiveCount').textContent = '-';
      document.getElementById('elapsed').textContent = '-';
  }

  setStatus(type, message) {
      const status = document.getElementById('status');
      status.className = `status ${type}`;
      status.textContent = message;
      
      // Control progress bar animation based on status
      const progressFill = document.getElementById('progressFill');
      if (type === 'rendering') {
          progressFill.classList.add('animating');
      } else {
          progressFill.classList.remove('animating');
      }
  }

  updateButtons() {
      document.getElementById('startBtn').disabled = this.isRendering;
      document.getElementById('stopBtn').disabled = !this.isRendering;
  }

  async handleImageClick(event) {
      const img = event.target;
      const rect = img.getBoundingClientRect();
      
      // Calculate click coordinates relative to the image
      const x = event.clientX - rect.left;
      const y = event.clientY - rect.top;
      
      // Convert to pixel coordinates based on image scale
      const scaleX = img.naturalWidth / rect.width;
      const scaleY = img.naturalHeight / rect.height;
      
      const pixelX = Math.floor(x * scaleX);
      const pixelY = Math.floor(y * scaleY);
      
      // Get current render parameters
      const params = this.getParameters();
      
      try {
          // Build inspect URL with scene-specific parameters
          const baseUrl = `/api/inspect?scene=${params.scene}&width=${params.width}&height=${params.height}&x=${pixelX}&y=${pixelY}`;
          const url = this.buildUrlWithSceneParams(baseUrl, params);
          
          const response = await fetch(url);
          
          if (response.ok) {
              const result = await response.json();
              this.displayInspectResult(result, pixelX, pixelY);
          } else {
              const error = await response.json();
              this.displayInspectError(error.error || 'Inspection failed');
          }
      } catch (error) {
          console.error('Inspection error:', error);
          this.displayInspectError('Network error during inspection');
      }
  }

  displayInspectResult(result, pixelX, pixelY) {
      const resultDiv = document.getElementById('inspectResult');
      
      if (!result.hit) {
          resultDiv.innerHTML = `
              <div class="inspect-card no-hit">
                  <div class="inspect-header">Pixel (${pixelX}, ${pixelY})</div>
                  <div>No object hit - background/sky</div>
              </div>
          `;
          return;
      }
      
      const materialProps = result.properties.material || {};
      const geometryProps = result.properties.geometry || {};
      
      let content = `
          <div class="inspect-card hit">
              <div class="inspect-header">Pixel (${pixelX}, ${pixelY}) - Object Hit</div>
              
              <div class="inspect-section">
                  <div class="inspect-property">
                      <span class="inspect-label">Material:</span>
                      <span class="inspect-value">${result.materialType}</span>
                  </div>
                  ${this.formatMaterialProperties(result.materialType, materialProps)}
              </div>
              
              <div class="inspect-section">
                  <div class="inspect-property">
                      <span class="inspect-label">Geometry:</span>
                      <span class="inspect-value">${result.geometryType}</span>
                  </div>
                  ${this.formatGeometryProperties(result.geometryType, geometryProps)}
              </div>
              
              <div class="inspect-section">
                  <div class="inspect-property">
                        <span class="inspect-label">Distance:</span>
                        <span class="inspect-value">${result.distance.toFixed(2)}</span>
                    </div>
                    <div class="inspect-property">
                        <span class="inspect-label">Position:</span>
                        <span class="inspect-value">(${result.point[0].toFixed(2)}, ${result.point[1].toFixed(2)}, ${result.point[2].toFixed(2)})</span>
                    </div>
                    <div class="inspect-property">
                        <span class="inspect-label">Normal:</span>
                        <span class="inspect-value">(${result.normal[0].toFixed(2)}, ${result.normal[1].toFixed(2)}, ${result.normal[2].toFixed(2)})</span>
                    </div>
                  <div class="inspect-property">
                      <span class="inspect-label">Front Face:</span>
                      <span class="inspect-value">${result.frontFace ? 'Yes' : 'No'}</span>
                  </div>
              </div>
          </div>
      `;
      
      resultDiv.innerHTML = content;
  }

  formatMaterialProperties(materialType, props) {
      let html = '';
      
      if (materialType === 'lambertian' && props.albedo) {
          html += this.createPropertyHTML('Albedo', props.albedo, props.color);
      }
      
      if (materialType === 'metal') {
          if (props.albedo) {
              html += this.createPropertyHTML('Albedo', props.albedo, props.color);
          }
                                if (props.fuzzness !== undefined) {
                html += this.createPropertyHTML('Fuzzness', props.fuzzness.toFixed(2));
            }
      }
      
                        if (materialType === 'dielectric' && props.refractiveIndex) {
            html += this.createPropertyHTML('Refractive Index', props.refractiveIndex.toFixed(2));
        }
      
      if (materialType === 'emissive' && props.emission) {
          html += this.createPropertyHTML('Emission', props.emission);
      }
      
      if (materialType === 'layered') {
          html += this.createPropertyHTML('Outer', props.outer?.type || 'unknown');
                                if (props.outer?.properties?.refractiveIndex) {
                html += `<div class="inspect-nested">`;
                html += this.createPropertyHTML('Refractive Index', props.outer.properties.refractiveIndex.toFixed(2));
                html += `</div>`;
            }
          
          html += this.createPropertyHTML('Inner', props.inner?.type || 'unknown');
          if (props.inner?.properties) {
              html += `<div class="inspect-nested">`;
              if (props.inner.properties.albedo) {
                  html += this.createPropertyHTML('Albedo', props.inner.properties.albedo, props.inner.properties.color);
              }
                                        if (props.inner.properties.fuzzness !== undefined) {
                    html += this.createPropertyHTML('Fuzzness', props.inner.properties.fuzzness.toFixed(2));
                }
              html += `</div>`;
          }
      }
      
      if (materialType === 'mixed') {
          if (props.description) {
              html += this.createPropertyHTML('Mix', props.description);
          }
          
          html += this.createPropertyHTML('Material 1', props.material1?.type || 'unknown');
          if (props.material1?.properties?.albedo) {
              html += `<div class="inspect-nested">`;
              html += this.createPropertyHTML('Albedo', props.material1.properties.albedo, props.material1.properties.color);
              html += `</div>`;
          }
          
          html += this.createPropertyHTML('Material 2', props.material2?.type || 'unknown');
          if (props.material2?.properties) {
              html += `<div class="inspect-nested">`;
              if (props.material2.properties.albedo) {
                  html += this.createPropertyHTML('Albedo', props.material2.properties.albedo, props.material2.properties.color);
              }
                                        if (props.material2.properties.fuzzness !== undefined) {
                    html += this.createPropertyHTML('Fuzzness', props.material2.properties.fuzzness.toFixed(2));
                }
              html += `</div>`;
          }
      }
      
      return html;
  }

  formatGeometryProperties(geometryType, props) {
      let html = '';
      
      if (geometryType === 'sphere') {
          if (props.center) {
              html += this.createPropertyHTML('Center', props.center);
          }
                                if (props.radius !== undefined) {
                html += this.createPropertyHTML('Radius', props.radius.toFixed(2));
            }
      }
      
      if (geometryType === 'quad' || geometryType === 'quad_light') {
          if (props.corner) {
              html += this.createPropertyHTML('Corner', props.corner);
          }
                                if (geometryType === 'quad_light' && props.area !== undefined) {
                html += this.createPropertyHTML('Area', props.area.toFixed(2));
            }
      }
      
      if (geometryType === 'plane') {
          if (props.point) {
              html += this.createPropertyHTML('Point', props.point);
          }
      }
      
      return html;
  }

  createPropertyHTML(label, value, color = null) {
      let valueStr = '';
      
      if (Array.isArray(value)) {
          valueStr = `(${value.map(v => v.toFixed(2)).join(', ')})`;
      } else {
          valueStr = value.toString();
      }
      
      let colorSwatch = '';
      if (color) {
          colorSwatch = `<span class="color-swatch" style="background: ${color};" title="Color: ${color}"></span>`;
      }
      
      return `
          <div class="inspect-property">
              <span class="inspect-label">${label}:</span>
              <span class="inspect-value">${valueStr}</span>
              ${colorSwatch}
          </div>
      `;
  }

  // Initialize tile streaming
  initializeTileStreaming(params) {
      const canvas = document.getElementById('renderCanvas');
      const noImage = document.getElementById('noImage');
      
      // Initialize render canvas
      this.renderCanvas = new RenderCanvas(canvas, 64);
      this.renderCanvas.initCanvas(parseInt(params.width), parseInt(params.height));
      
      canvas.style.display = 'block';
      noImage.style.display = 'none';
      
      // Add click handler for canvas
      canvas.onclick = (event) => this.handleCanvasClick(event);
  }

  // Handle tile updates in streaming mode
  updateTile(data) {
      if (this.renderCanvas) {
          this.renderCanvas.updateTile(data.tileX, data.tileY, `data:image/png;base64,${data.imageData}`);
      }
  }

  // Handle pass completion in streaming mode
  updatePassComplete(data) {
      // Update progress bar
      const progress = (data.passNumber / data.totalPasses) * 100;
      document.getElementById('progressFill').style.width = `${progress}%`;
      
      // Update stats
      document.getElementById('currentPass').textContent = data.passNumber;
      document.getElementById('totalPasses').textContent = data.totalPasses;
      document.getElementById('elapsed').textContent = `${(data.elapsedMs / 1000).toFixed(1)}s`;
      
      // Update render statistics if available
      if (data.totalPixels !== undefined) {
          document.getElementById('totalPixels').textContent = data.totalPixels.toLocaleString();
      }
      if (data.totalSamples !== undefined) {
          document.getElementById('totalSamples').textContent = data.totalSamples.toLocaleString();
      }
      if (data.averageSamples !== undefined) {
          document.getElementById('avgSamples').textContent = data.averageSamples.toFixed(1);
      }
      if (data.primitiveCount !== undefined) {
          document.getElementById('primitiveCount').textContent = data.primitiveCount.toLocaleString();
      }
      
      // Update status
      this.setStatus('rendering', `Pass ${data.passNumber}/${data.totalPasses} completed`);
  }

  // Handle canvas clicks for pixel inspection
  handleCanvasClick(event) {
      if (!this.renderCanvas) return;
      
      this.renderCanvas.handleClick(event, (x, y) => {
          this.handleImageClick({
              target: event.target,
              clientX: event.clientX,
              clientY: event.clientY
          });
      });
  }

  displayInspectError(errorMessage) {
      const resultDiv = document.getElementById('inspectResult');
      resultDiv.innerHTML = `
          <div class="inspect-card error">
              <div class="inspect-header">Inspection Error</div>
              <div>${errorMessage}</div>
          </div>
      `;
  }
}

// Initialize the application
new ProgressiveRaytracer();
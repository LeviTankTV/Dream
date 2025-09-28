class GameClient {
    constructor() {
        this.ws = null;
        this.gameState = {
            players: {},
            myId: null,
            worldSize: 2000
        };
        this.canvas = null;
        this.ctx = null;
        this.playerRadius = 15;
        this.keys = {};
        this.camera = { x: 0, y: 0 };
        this.gameLoopInterval = null;
        this.lastUpdateTime = 0;
        this.fps = 60;
        this.frameTime = 1000 / this.fps;

        this.authToken = localStorage.getItem('authToken');
        this.username = localStorage.getItem('username');
        
        this.init();
    }

    init() {
    const loadingOverlay = document.getElementById('loadingOverlay');
    
    // Ensure it's visible at start
    if (loadingOverlay) {
        loadingOverlay.classList.remove('hidden');
    }

    this.canvas = document.getElementById('gameCanvas');
    this.ctx = this.canvas.getContext('2d');
    this.resizeCanvas();
    
    window.addEventListener('resize', () => this.resizeCanvas());
    this.setupControls();
    this.setupAuthHandlers();
    
    if (this.authToken && this.username) {
        document.getElementById('authOverlay').style.display = 'none';
    } else {
        this.showAuth();
    }
    
    this.startGameLoop();
    
    console.log('Game client initialized');

    // Hide loading screen after at least 1 second with smooth fade
    setTimeout(() => {
        if (loadingOverlay) {
            loadingOverlay.classList.add('hidden');
            // Optional: remove from DOM after animation completes
            setTimeout(() => {
                if (loadingOverlay.parentNode) {
                    loadingOverlay.style.display = 'none';
                }
            }, 400); // match transition duration
        }
    }, 1000);
    }
    
    setupAuthHandlers() {
        // Добавляем проверку на существование элементов
        const loginBtn = document.getElementById('loginBtn');
        const registerBtn = document.getElementById('registerBtn');
        const passwordInput = document.getElementById('passwordInput');
        const enterGameBtn = document.getElementById('enterGame');

        if (loginBtn) {
            loginBtn.addEventListener('click', () => this.login());
        }
        if (registerBtn) {
            registerBtn.addEventListener('click', () => this.register());
        }
        if (passwordInput) {
            passwordInput.addEventListener('keypress', (e) => {
                if (e.key === 'Enter') this.login();
            });
        }
        if (enterGameBtn) {
            enterGameBtn.addEventListener('click', () => this.enterGame());
        }
    }

     async login() {
        const username = document.getElementById('usernameInput').value;
        const password = document.getElementById('passwordInput').value;
        
        if (!username || !password) {
            this.showAuthMessage('Введите логин и пароль');
            return;
        }

        try {
            const response = await fetch('/api/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ 
                    login: username, 
                    password: password 
                })
            });
            
            const result = await response.json();
            
            if (result.success) {
                this.authToken = result.user_id; // Используем user_id как токен
                this.username = username;
                
                // Сохраняем в localStorage
                localStorage.setItem('authToken', this.authToken);
                localStorage.setItem('username', this.username);
                
                this.hideAuth();
            } else {
                this.showAuthMessage('Ошибка входа: ' + result.message);
            }
        } catch (error) {
            this.showAuthMessage('Ошибка соединения: ' + error.message);
        }
    }

    async register() {
        const username = document.getElementById('usernameInput').value;
        const password = document.getElementById('passwordInput').value;
        
        if (!username || !password) {
            this.showAuthMessage('Введите логин и пароль');
            return;
        }

        try {
            const response = await fetch('/api/register', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ 
                    login: username, 
                    password: password 
                })
            });
            
            const result = await response.json();
            
            if (result.success) {
                this.showAuthMessage('Регистрация успешна! Теперь войдите.');
                document.getElementById('passwordInput').value = ''; // Очищаем пароль
            } else {
                this.showAuthMessage('Ошибка регистрации: ' + result.message);
            }
        } catch (error) {
            this.showAuthMessage('Ошибка соединения: ' + error.message);
        }
    }

    showAuth() {
        document.getElementById('authOverlay').style.display = 'flex';
    }

    hideAuth() {
        document.getElementById('authOverlay').style.display = 'none';
    }

    showAuthMessage(message) {
        document.getElementById('authMessage').textContent = message;
    }


    startGameLoop() {
        this.stopGameLoop();
        
        this.gameLoopInterval = setInterval(() => {
            this.gameLoop();
        }, this.frameTime);
        
        this.movementInterval = setInterval(() => {
            this.handleMovement();
        }, 1000/120);
    }

    stopGameLoop() {
        if (this.gameLoopInterval) {
            clearInterval(this.gameLoopInterval);
            this.gameLoopInterval = null;
        }
        if (this.movementInterval) {
            clearInterval(this.movementInterval);
            this.movementInterval = null;
        }
    }

    resizeCanvas() {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
    }

    setupControls() {
        document.addEventListener('keydown', (e) => {
            if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', ' ', 'w', 'a', 's', 'd', "ц", 'ф', "ы", "в"].includes(e.key)) {
                e.preventDefault();
            }
            this.keys[e.key.toLowerCase()] = true;
        });
        
        document.addEventListener('keyup', (e) => {
            this.keys[e.key.toLowerCase()] = false;
        });

        window.addEventListener('blur', () => {
            this.keys = {};
        });
    }

    enterGame() {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        console.log('Already connected to game server');
        return;
    }

    // Убедимся, что токен есть
    if (!this.authToken) {
        this.showAuth();
        return;
    }

    const wsUrl = `ws://localhost:8080/ws?token=${this.authToken}`;
    this.ws = new WebSocket(wsUrl);
    
    this.ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            if (data.type === "error") {
                if (data.message.includes('auth')) {
                    this.handleAuthError();
                }
                alert("Connection rejected: " + data.message);
                this.ws.close();
                return;
            }
            this.handleGameMessage(data);
        } catch (error) {
            console.error('Error parsing game message:', error);
        }
    };
    
    this.ws.onclose = () => {
        console.log('Disconnected from game server');
        this.showAuth(); // или просто покажем кнопку снова
        this.hideDebugInfo();
        document.getElementById('enterGame').style.display = 'block';
        
        if (this.keepAliveInterval) {
            clearInterval(this.keepAliveInterval);
            this.keepAliveInterval = null;
        }
    };
    
    this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };

    this.keepAliveInterval = setInterval(() => {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ type: 'ping' }));
        }
    }, 30000);
}
     handleAuthError() {
        localStorage.removeItem('authToken');
        localStorage.removeItem('username');
        this.authToken = null;
        this.username = null;
    
        this.hideDebugInfo();
        document.getElementById('enterGame').style.display = 'block';
        this.showAuth();
        this.showAuthMessage('Ошибка аутентификации. Войдите снова.');
    }

    handleGameMessage(data) {
    switch (data.type) {
        case 'state':
            this.gameState.players = data.players;
            this.gameState.myId = data.yourId;
            this.gameState.worldSize = data.worldSize || 1000;
            
            // Показываем debug-панель и скрываем кнопку "Enter Game"
            this.showDebugInfo();
            document.getElementById('enterGame').style.display = 'none';
            
            this.updateUI();
            break;
        case 'pong':
            break;
    }
}

    showDebugInfo() {
        document.getElementById('debug-info').style.display = 'block';
    }

    hideDebugInfo() {
        document.getElementById('debug-info').style.display = 'none';
    }

     updateUI() {
        const myPlayer = this.gameState.players[this.gameState.myId];
        if (myPlayer) {
            document.getElementById('coordinates').textContent = 
                `Your coordinates: ${Math.round(myPlayer.x)}, ${Math.round(myPlayer.y)}`;
            document.getElementById('playerCount').textContent = 
                Object.keys(this.gameState.players).length;
            
            // Показываем имя пользователя, если есть
            if (this.username) {
                document.getElementById('playerId').textContent = this.username;
            }
        }
    }

    handleMovement() {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN || !this.gameState.myId) return;
        
        let dx = 0, dy = 0;
        const speed = 5;
        
        if (this.keys['arrowup'] || this.keys['w'] || this.keys['ц']) dy = -speed;
        if (this.keys['arrowdown'] || this.keys['s'] || this.keys['ы']) dy = speed;
        if (this.keys['arrowleft'] || this.keys['a'] || this.keys['ф']) dx = -speed;
        if (this.keys['arrowright'] || this.keys['d'] || this.keys['в']) dx = speed;
        
        if (dx !== 0 && dy !== 0) {
            dx *= 0.707;
            dy *= 0.707;
        }
        
        if (dx !== 0 || dy !== 0) {
            this.ws.send(JSON.stringify({
                type: 'move',
                data: { dx, dy }
            }));
        }
    }

    gameLoop() {
        const currentTime = Date.now();
        
        if (currentTime - this.lastUpdateTime < this.frameTime) {
            return;
        }
        
        this.lastUpdateTime = currentTime;
        
        // Clear canvas with WHITE background (as requested)
        this.ctx.fillStyle = '#ffffff';
        this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
        
        this.drawWorld();
    }

    drawWorld() {
        const myPlayer = this.gameState.players[this.gameState.myId];
        if (!myPlayer) return;
        
        // Center camera on player
        this.camera.x = -myPlayer.x + this.canvas.width / 2;
        this.camera.y = -myPlayer.y + this.canvas.height / 2;
        
        // Draw the world square with black borders
        this.drawWorldSquare();
        
        // Draw grid
        this.drawWorldGrid();
        
        // Draw players
        this.drawPlayers();
    }

    drawWorldSquare() {
        // Draw the main world square (1000x1000) filled with white and black borders
        this.ctx.fillStyle = '#ffffff'; // White fill
        this.ctx.fillRect(
            this.camera.x, 
            this.camera.y, 
            this.gameState.worldSize, 
            this.gameState.worldSize
        );
        
        // Draw black border around the world
        this.ctx.strokeStyle = '#000000'; // Black border
        this.ctx.lineWidth = 4;
        this.ctx.strokeRect(
            this.camera.x, 
            this.camera.y, 
            this.gameState.worldSize, 
            this.gameState.worldSize
        );
    }

    drawWorldGrid() {
        this.ctx.strokeStyle = 'rgba(0, 0, 0, 0.1)'; // Light gray grid on white background
        this.ctx.lineWidth = 1;
        
        const gridSize = 100;
        
        // Calculate grid starting positions in world coordinates
        const worldStartX = Math.floor(0 / gridSize) * gridSize;
        const worldStartY = Math.floor(0 / gridSize) * gridSize;
        const worldEndX = this.gameState.worldSize;
        const worldEndY = this.gameState.worldSize;
        
        // Draw vertical grid lines
        for (let x = worldStartX; x <= worldEndX; x += gridSize) {
            const screenX = x + this.camera.x;
            this.ctx.beginPath();
            this.ctx.moveTo(screenX, this.camera.y);
            this.ctx.lineTo(screenX, this.camera.y + this.gameState.worldSize);
            this.ctx.stroke();
        }
        
        // Draw horizontal grid lines
        for (let y = worldStartY; y <= worldEndY; y += gridSize) {
            const screenY = y + this.camera.y;
            this.ctx.beginPath();
            this.ctx.moveTo(this.camera.x, screenY);
            this.ctx.lineTo(this.camera.x + this.gameState.worldSize, screenY);
            this.ctx.stroke();
        }
    }

    drawPlayers() {
        Object.values(this.gameState.players).forEach(player => {
            const screenX = player.x + this.camera.x;
            const screenY = player.y + this.camera.y;
            
            // Only draw players within the world bounds
            if (player.x < 0 || player.x > this.gameState.worldSize || 
                player.y < 0 || player.y > this.gameState.worldSize) {
                return;
            }
            
            // Draw player circle
            this.ctx.beginPath();
            this.ctx.arc(screenX, screenY, this.playerRadius, 0, 2 * Math.PI);
            this.ctx.fillStyle = player.color || '#4CAF50'; // Default green if no color
            this.ctx.fill();
            
            // White border around player
            this.ctx.strokeStyle = '#ffffff';
            this.ctx.lineWidth = 2;
            this.ctx.stroke();
            
            // Player ID label
            this.ctx.fillStyle = '#000000'; // Black text on white background
            this.ctx.font = '12px Arial';
            this.ctx.textAlign = 'center';
            this.ctx.fillText(player.id.substring(0, 6), screenX, screenY - this.playerRadius - 8);
            
            // Highlight own player with green outline
            if (player.id === this.gameState.myId) {
                this.ctx.beginPath();
                this.ctx.arc(screenX, screenY, this.playerRadius + 5, 0, 2 * Math.PI);
                this.ctx.strokeStyle = '#00ff00';
                this.ctx.lineWidth = 3;
                this.ctx.stroke();
                
                this.drawMinimap(player);
            }
        });
    }

    drawMinimap(player) {
        const minimapSize = 120;
        const padding = 10;
        
        this.ctx.save();
        
        // Minimap background
        this.ctx.fillStyle = 'rgba(255, 255, 255, 0.9)'; // White with slight transparency
        this.ctx.fillRect(padding, padding, minimapSize, minimapSize);
        this.ctx.strokeStyle = '#000000';
        this.ctx.lineWidth = 2;
        this.ctx.strokeRect(padding, padding, minimapSize, minimapSize);
        
        // Scale for minimap
        const scale = minimapSize / this.gameState.worldSize;
        
        // Draw world border on minimap
        this.ctx.strokeStyle = '#000000';
        this.ctx.lineWidth = 1;
        this.ctx.strokeRect(padding, padding, minimapSize, minimapSize);
        
        // Draw players on minimap
        Object.values(this.gameState.players).forEach(p => {
            if (p.x >= 0 && p.x <= this.gameState.worldSize && 
                p.y >= 0 && p.y <= this.gameState.worldSize) {
                const mapX = padding + p.x * scale;
                const mapY = padding + p.y * scale;
                const mapRadius = Math.max(2, 3); // Small fixed radius for minimap
                
                this.ctx.beginPath();
                this.ctx.arc(mapX, mapY, mapRadius, 0, 2 * Math.PI);
                this.ctx.fillStyle = p.color || '#4CAF50';
                this.ctx.fill();
                
                if (p.id === this.gameState.myId) {
                    this.ctx.strokeStyle = '#00ff00';
                    this.ctx.lineWidth = 2;
                    this.ctx.stroke();
                }
            }
        });
        
        this.ctx.restore();
    }

    destroy() {
        this.stopGameLoop();
        if (this.keepAliveInterval) {
            clearInterval(this.keepAliveInterval);
        }
        if (this.ws) {
            this.ws.close();
        }
    }
}

window.addEventListener('load', () => {
    window.gameClient = new GameClient();
});

window.addEventListener('beforeunload', () => {
    if (window.gameClient) {
        window.gameClient.destroy();
    }
});

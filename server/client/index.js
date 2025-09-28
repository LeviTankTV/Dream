class GameClient {
    constructor() {
        this.ws = null;
        this.gameState = {
            players: {},
            myId: null,
            worldSize: 1000
        };
        this.canvas = null;
        this.ctx = null;
        this.playerRadius = 15;
        this.keys = {};
        this.camera = { x: 0, y: 0 };
        this.gameLoopInterval = null;
        this.lastUpdateTime = 0;
        this.fps = 60; // Желаемый FPS
        this.frameTime = 1000 / this.fps;
        
        this.init();
    }

    init() {
        this.canvas = document.getElementById('gameCanvas');
        this.ctx = this.canvas.getContext('2d');
        this.resizeCanvas();
        
        window.addEventListener('resize', () => this.resizeCanvas());
        this.setupControls();
        
        document.getElementById('enterGame').addEventListener('click', () => this.enterGame());
        
        // Запускаем игровой цикл на setInterval вместо requestAnimationFrame
        this.startGameLoop();
        
        console.log('Game client initialized');
    }

    startGameLoop() {
        // Останавливаем предыдущий цикл, если был
        this.stopGameLoop();
        
        // Запускаем новый цикл
        this.gameLoopInterval = setInterval(() => {
            this.gameLoop();
        }, this.frameTime);
        
        // Также запускаем цикл движения с более высокой частотой для плавности
        this.movementInterval = setInterval(() => {
            this.handleMovement();
        }, 1000/120); // 120 FPS для обработки ввода
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
            // Предотвращаем стандартное поведение браузера для игровых клавиш
            if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', ' ', 'w', 'a', 's', 'd'].includes(e.key)) {
                e.preventDefault();
            }
            this.keys[e.key.toLowerCase()] = true;
        });
        
        document.addEventListener('keyup', (e) => {
            this.keys[e.key.toLowerCase()] = false;
        });

        // Обработчик потери фокуса - сбрасываем состояние клавиш
        window.addEventListener('blur', () => {
            this.keys = {};
        });
    }

    enterGame() {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            console.log('Already connected to game server');
            return;
        }

        this.ws = new WebSocket('ws://localhost:8080/ws');
        
        this.ws.onopen = () => {
            console.log('Connected to game server');
            this.showGameUI();
            // Игровой цикл уже запущен в init()
        };
        
        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.handleGameMessage(data);
            } catch (error) {
                console.error('Error parsing game message:', error);
            }
        };
        
        this.ws.onclose = () => {
            console.log('Disconnected from game server');
            this.showLoginUI();
        };
        
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        // Пинг-понг для поддержания соединения
        this.keepAliveInterval = setInterval(() => {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                this.ws.send(JSON.stringify({ type: 'ping' }));
            }
        }, 30000);
    }

    handleGameMessage(data) {
        switch (data.type) {
            case 'state':
                this.gameState.players = data.players;
                this.gameState.myId = data.yourId;
                this.gameState.worldSize = data.worldSize || 1000;
                this.updateUI();
                break;
            case 'pong':
                // Обработка пинг-понга для поддержания соединения
                break;
        }
    }

    showGameUI() {
        document.getElementById('playerInfo').style.display = 'block';
        document.getElementById('enterGame').style.display = 'none';
    }

    showLoginUI() {
        document.getElementById('playerInfo').style.display = 'none';
        document.getElementById('enterGame').style.display = 'block';
        
        // Очищаем интервал пинг-понга при отключении
        if (this.keepAliveInterval) {
            clearInterval(this.keepAliveInterval);
        }
    }

    updateUI() {
        const myPlayer = this.gameState.players[this.gameState.myId];
        if (myPlayer) {
            document.getElementById('coordinates').textContent = 
                `Your coordinates: ${Math.round(myPlayer.x)}, ${Math.round(myPlayer.y)}`;
            document.getElementById('playerCount').textContent = 
                Object.keys(this.gameState.players).length;
        }
    }

    handleMovement() {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN || !this.gameState.myId) return;
        
        let dx = 0, dy = 0;
        const speed = 5;
        
        if (this.keys['arrowup'] || this.keys['w']) dy = -speed;
        if (this.keys['arrowdown'] || this.keys['s']) dy = speed;
        if (this.keys['arrowleft'] || this.keys['a']) dx = -speed;
        if (this.keys['arrowright'] || this.keys['d']) dx = speed;
        
        // Диагональное движение - нормализуем скорость
        if (dx !== 0 && dy !== 0) {
            dx *= 0.707; // 1/√2
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
        
        // Ограничиваем FPS для производительности
        if (currentTime - this.lastUpdateTime < this.frameTime) {
            return;
        }
        
        this.lastUpdateTime = currentTime;
        
        // Очищаем canvas
        this.ctx.fillStyle = '#162447';
        this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
        
        // Рисуем игровой мир
        this.drawWorld();
    }

    drawWorld() {
        const myPlayer = this.gameState.players[this.gameState.myId];
        if (!myPlayer) return;
        
        // Центрируем камеру на своем игроке
        this.camera.x = this.canvas.width / 2 - myPlayer.x;
        this.camera.y = this.canvas.height / 2 - myPlayer.y;
        
        // Рисуем сетку мира
        this.drawWorldGrid();
        
        // Рисуем всех игроков
        this.drawPlayers();
        
        // Рисуем границы мира
        this.drawWorldBounds();
    }

    drawWorldGrid() {
        this.ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
        this.ctx.lineWidth = 1;
        
        const gridSize = 100;
        const startX = Math.floor(-this.camera.x / gridSize) * gridSize;
        const startY = Math.floor(-this.camera.y / gridSize) * gridSize;
        
        for (let x = startX; x < this.canvas.width; x += gridSize) {
            this.ctx.beginPath();
            this.ctx.moveTo(x, 0);
            this.ctx.lineTo(x, this.canvas.height);
            this.ctx.stroke();
        }
        
        for (let y = startY; y < this.canvas.height; y += gridSize) {
            this.ctx.beginPath();
            this.ctx.moveTo(0, y);
            this.ctx.lineTo(this.canvas.width, y);
            this.ctx.stroke();
        }
    }

    drawPlayers() {
        Object.values(this.gameState.players).forEach(player => {
            const screenX = player.x + this.camera.x;
            const screenY = player.y + this.camera.y;
            
            // Проверяем, находится ли игрок в области видимости
            if (screenX < -this.playerRadius || screenX > this.canvas.width + this.playerRadius ||
                screenY < -this.playerRadius || screenY > this.canvas.height + this.playerRadius) {
                return; // Пропускаем отрисовку если игрок за пределами экрана
            }
            
            // Рисуем кружок игрока
            this.ctx.beginPath();
            this.ctx.arc(screenX, screenY, this.playerRadius, 0, 2 * Math.PI);
            this.ctx.fillStyle = player.color;
            this.ctx.fill();
            
            // Обводка
            this.ctx.strokeStyle = '#ffffff';
            this.ctx.lineWidth = 2;
            this.ctx.stroke();
            
            // Подпись с ID
            this.ctx.fillStyle = '#ffffff';
            this.ctx.font = '12px Arial';
            this.ctx.textAlign = 'center';
            this.ctx.fillText(player.id.substring(0, 6), screenX, screenY - this.playerRadius - 8);
            
            // Выделяем своего игрока
            if (player.id === this.gameState.myId) {
                this.ctx.beginPath();
                this.ctx.arc(screenX, screenY, this.playerRadius + 5, 0, 2 * Math.PI);
                this.ctx.strokeStyle = '#00ff00';
                this.ctx.lineWidth = 3;
                this.ctx.stroke();
                
                // Миникарта для своего игрока
                this.drawMinimap(player);
            }
        });
    }

    drawWorldBounds() {
        const boundsColor = 'rgba(255, 100, 100, 0.3)';
        const boundsWidth = 20;
        
        // Левая граница
        if (this.camera.x < 0) {
            this.ctx.fillStyle = boundsColor;
            this.ctx.fillRect(0, 0, -this.camera.x, this.canvas.height);
        }
        
        // Правая граница
        const rightBound = this.gameState.worldSize + this.camera.x;
        if (rightBound < this.canvas.width) {
            this.ctx.fillStyle = boundsColor;
            this.ctx.fillRect(rightBound, 0, this.canvas.width - rightBound, this.canvas.height);
        }
        
        // Верхняя граница
        if (this.camera.y < 0) {
            this.ctx.fillStyle = boundsColor;
            this.ctx.fillRect(0, 0, this.canvas.width, -this.camera.y);
        }
        
        // Нижняя граница
        const bottomBound = this.gameState.worldSize + this.camera.y;
        if (bottomBound < this.canvas.height) {
            this.ctx.fillStyle = boundsColor;
            this.ctx.fillRect(0, bottomBound, this.canvas.width, this.canvas.height - bottomBound);
        }
    }

    drawMinimap(player) {
        const minimapSize = 120;
        const padding = 10;
        
        this.ctx.save();
        
        // Фон миникарты
        this.ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
        this.ctx.fillRect(padding, padding, minimapSize, minimapSize);
        this.ctx.strokeStyle = '#ffffff';
        this.ctx.lineWidth = 2;
        this.ctx.strokeRect(padding, padding, minimapSize, minimapSize);
        
        // Масштаб для миникарты
        const scale = minimapSize / this.gameState.worldSize;
        
        // Рисуем игроков на миникарте
        Object.values(this.gameState.players).forEach(p => {
            const mapX = padding + p.x * scale;
            const mapY = padding + p.y * scale;
            const mapRadius = Math.max(3, this.playerRadius * scale);
            
            this.ctx.beginPath();
            this.ctx.arc(mapX, mapY, mapRadius, 0, 2 * Math.PI);
            this.ctx.fillStyle = p.color;
            this.ctx.fill();
            
            if (p.id === this.gameState.myId) {
                this.ctx.strokeStyle = '#00ff00';
                this.ctx.lineWidth = 2;
                this.ctx.stroke();
            }
        });
        
        this.ctx.restore();
    }

    // Метод для очистки ресурсов
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

// Инициализация игры при загрузке страницы
window.addEventListener('load', () => {
    window.gameClient = new GameClient();
});

// Очистка ресурсов при закрытии страницы
window.addEventListener('beforeunload', () => {
    if (window.gameClient) {
        window.gameClient.destroy();
    }
});
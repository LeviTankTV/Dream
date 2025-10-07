package game

import (
    "fmt"
    "runtime"
    "time"
)

// Добавляем структуру для отслеживания блокировок
type DeadlockDetector struct {
    lastBroadcast time.Time
    threshold     time.Duration
}

func NewDeadlockDetector() *DeadlockDetector {
    return &DeadlockDetector{
        threshold: 10 * time.Second,
    }
}

func (d *DeadlockDetector) Check() {
    if time.Since(d.lastBroadcast) > d.threshold {
        fmt.Printf("🚨 DEADLOCK DETECTED: No broadcast for %v\n", time.Since(d.lastBroadcast))
        
        // Печатаем stack trace всех горутин
        buf := make([]byte, 1<<20)
        stackLen := runtime.Stack(buf, true)
        fmt.Printf("=== ALL GOROUTINE STACKS ===\n%s\n=== END STACKS ===\n", buf[:stackLen])
        
        // Сбрасываем таймер
        d.lastBroadcast = time.Now()
    }
}

func (d *DeadlockDetector) UpdateBroadcast() {
    d.lastBroadcast = time.Now()
}
package game

import (
	"math"
	"time"
)

type Player struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Username       string    `json:"username"`
	X              float64   `json:"x"`
	Y              float64   `json:"y"`
	Color          string    `json:"color"`
	Speed          float64   `json:"speed"`
	PortalCooldown time.Time `json:"-"`
	CurrentZone    string    `json:"currentZone"`
	Radius         float64   `json:"radius"`

	Health          int       `json:"health"`
	MaxHealth       int       `json:"max_health"`
	CollisionDamage int       `json:"collision_damage"`
	LastHitTime     time.Time `json:"-"` // Время последнего получения урона
}

func NewPlayer(id, userID, username string, x, y float64, color string) *Player {
	return &Player{
		ID:              id,
		UserID:          userID,
		Username:        username,
		X:               x,
		Y:               y,
		Color:           color,
		Speed:           2.5,
		Radius:          15.0,
		Health:          100,
		MaxHealth:       100,
		CollisionDamage: 25, // Базовый урон игрока
		LastHitTime:     time.Now(),
	}
}

func (p *Player) DistanceTo(otherX, otherY float64) float64 {
	dx := p.X - otherX
	dy := p.Y - otherY
	return math.Sqrt(dx*dx + dy*dy)
}

func (p *Player) TakeDamage(damage int) bool {
	now := time.Now()
	if now.Sub(p.LastHitTime) < 100*time.Millisecond {
		return false // Слишком рано для следующего удара
	}

	p.Health -= damage
	p.LastHitTime = now

	if p.Health < 0 {
		p.Health = 0
	}

	return true
}

// IsAlive проверяет, жив ли игрок
func (p *Player) IsAlive() bool {
	return p.Health > 0
}

// Respawn возрождает игрока
func (p *Player) Respawn(x, y float64) {
	p.Health = p.MaxHealth
	p.X = x
	p.Y = y
	p.LastHitTime = time.Now()
}

// CanAttack проверяет, может ли игрок атаковать
func (p *Player) CanAttack() bool {
	return time.Since(p.LastHitTime) >= 100*time.Millisecond
}

// MarkAttack отмечает время атаки
func (p *Player) MarkAttack() {
	p.LastHitTime = time.Now()
}

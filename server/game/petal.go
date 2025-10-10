package game

import (
	"fmt"
	"math"
	"time"
)

type PetalType string

const (
	PetalTypeWolf   PetalType = "wolf"
	PetalTypeGoblin PetalType = "goblin"
	PetalTypeOrc    PetalType = "orc"
)

type Petal struct {
	ID         string    `json:"id"`
	Type       PetalType `json:"type"`
	Health     int       `json:"health"`
	MaxHealth  int       `json:"max_health"`
	Damage     int       `json:"damage"`
	HealAmount int       `json:"heal_amount"`
	HealRate   float64   `json:"heal_rate"` // seconds between heals
	Radius     float64   `json:"radius"`    // orbit radius
	Angle      float64   `json:"angle"`     // current orbit angle
	Speed      float64   `json:"speed"`     // orbit speed
	OwnerID    string    `json:"owner_id"`
	IsActive   bool      `json:"is_active"`
	LastHeal   time.Time `json:"-"`
	LastAttack time.Time `json:"-"`
	X          float64   `json:"x"` // current x position
	Y          float64   `json:"y"`
}

// Конфигурация лепестков
var PetalConfigs = map[PetalType]struct {
	Health     int
	Damage     int
	HealAmount int
	HealRate   float64
	Radius     float64
	Speed      float64
}{
	PetalTypeWolf: {
		Health:     50,
		Damage:     0,
		HealAmount: 5,
		HealRate:   2.0, // heal every 2 seconds
		Radius:     60,
		Speed:      1.5,
	},
	PetalTypeGoblin: {
		Health:     15,
		Damage:     20,
		HealAmount: 0,
		HealRate:   0,
		Radius:     50,
		Speed:      2.0,
	},
	PetalTypeOrc: {
		Health:     20,
		Damage:     12,
		HealAmount: 0,
		HealRate:   0,
		Radius:     70,
		Speed:      1.2,
	},
}

func NewPetal(petalType PetalType, ownerID string) *Petal {
	config := PetalConfigs[petalType]

	return &Petal{
		ID:         fmt.Sprintf("petal_%s_%d", petalType, time.Now().UnixNano()),
		Type:       petalType,
		Health:     config.Health,
		MaxHealth:  config.Health,
		Damage:     config.Damage,
		HealAmount: config.HealAmount,
		HealRate:   config.HealRate,
		Radius:     config.Radius,
		Angle:      0,
		Speed:      config.Speed,
		OwnerID:    ownerID,
		IsActive:   true,
		LastHeal:   time.Now(),
		LastAttack: time.Now(),
	}
}

func (p *Petal) UpdatePosition(playerX, playerY float64, deltaTime float64) (float64, float64) {
	p.Angle += p.Speed * deltaTime
	if p.Angle > 2*math.Pi {
		p.Angle -= 2 * math.Pi
	}

	x := playerX + p.Radius*math.Cos(p.Angle)
	y := playerY + p.Radius*math.Sin(p.Angle)

	return x, y
}

func (p *Petal) CanHeal() bool {
	return p.IsActive && p.HealAmount > 0 && time.Since(p.LastHeal) >= time.Duration(p.HealRate*float64(time.Second))
}

func (p *Petal) CanAttack() bool {
	return p.IsActive && p.Damage > 0 && time.Since(p.LastAttack) >= 500*time.Millisecond
}

func (p *Petal) TakeDamage(damage int) {
	p.Health -= damage
	if p.Health <= 0 {
		p.Health = 0
		p.IsActive = false
	}
}

func (p *Petal) IsAlive() bool {
	return p.Health > 0
}

func (p *Petal) Respawn() {
	p.Health = p.MaxHealth
	p.IsActive = true
}

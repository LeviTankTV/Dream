package game

import (
	"math"
	"math/rand"
	"time"
)

type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityUncommon  Rarity = "uncommon"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

// Множители характеристик для редкостей
var RarityMultipliers = map[Rarity]struct {
	HealthMultiplier float64
	DamageMultiplier float64
	RadiusMultiplier float64
	SpeedMultiplier  float64
}{
	RarityCommon:    {1.0, 1.0, 1.0, 1.0},
	RarityUncommon:  {1.4, 1.3, 1.4, 1.1},
	RarityRare:      {1.8, 1.6, 1.8, 1.2},
	RarityEpic:      {2.25, 2.0, 2.25, 1.3},
	RarityLegendary: {6.75, 6.0, 6.75, 1.5}, // В 3 раза больше эпика
}

// Распределение редкостей по зонам
var ZoneRarityDistribution = map[string]map[Rarity]float64{
	"common": {
		RarityCommon:   0.8,
		RarityUncommon: 0.2,
	},
	"uncommon": {
		RarityCommon:   0.5,
		RarityUncommon: 0.4,
		RarityRare:     0.1,
	},
	"rare": {
		RarityCommon:   0.2,
		RarityUncommon: 0.6,
		RarityRare:     0.18,
		RarityEpic:     0.02,
	},
	"epic": {
		RarityCommon:   0.05,
		RarityUncommon: 0.5,
		RarityRare:     0.4,
		RarityEpic:     0.05,
	},
	"legendary": {
		RarityCommon:    0.99,
		RarityLegendary: 0.01,
	},
}

type MobType string

const (
	MobTypeGoblin MobType = "goblin"
	MobTypeOrc    MobType = "orc"
	MobTypeWolf   MobType = "wolf"
)

type MobState string

const (
	MobStateWandering MobState = "wandering"
	MobStateChasing   MobState = "chasing"
	MobStateAttacking MobState = "attacking"
	MobStateFleeing   MobState = "fleeing"
)

// Константы для разных типов мобов
var MobConfigs = map[MobType]struct {
	Health         int
	Damage         int
	Speed          float64
	Radius         float64
	DetectionRange float64
}{
	MobTypeGoblin: {Health: 30, Damage: 8, Speed: 10.0, Radius: 7.0, DetectionRange: 500},
	MobTypeOrc:    {Health: 80, Damage: 15, Speed: 20.0, Radius: 25.0, DetectionRange: 500},
	MobTypeWolf:   {Health: 40, Damage: 10, Speed: 15.0, Radius: 12.0, DetectionRange: 500},
}

type Mob struct {
	ID             string  `json:"id"`
	Type           MobType `json:"type"`
	Rarity         Rarity  `json:"rarity"`
	Health         int     `json:"health"`
	MaxHealth      int     `json:"max_health"`
	Damage         int     `json:"damage"`
	Speed          float64 `json:"speed"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Zone           string  `json:"zone"`
	Radius         float64 `json:"radius"`
	DetectionRange float64 `json:"-"`

	// Поведение
	TargetX        float64   `json:"-"`
	TargetY        float64   `json:"-"`
	LastMoveTime   time.Time `json:"-"`
	State          MobState  `json:"-"`
	TargetPlayer   string    `json:"-"`
	AttackCooldown time.Time `json:"-"`
	CreationTime   time.Time `json:"-"`
	LastHitTime    time.Time `json:"-"`
}

func getRandomRarity(zone string) Rarity {
	distribution, exists := ZoneRarityDistribution[zone]
	if !exists {
		return RarityCommon
	}

	// Генерируем случайное число от 0 до 1
	r := rand.Float64()
	cumulative := 0.0

	for rarity, probability := range distribution {
		cumulative += probability
		if r <= cumulative {
			return rarity
		}
	}

	return RarityCommon
}

func applyRarityMultipliers(baseConfig MobType, rarity Rarity, zone string) (int, int, float64, float64) {
	config := MobConfigs[baseConfig]
	multiplier := RarityMultipliers[rarity]

	// Рассчитываем итоговые характеристики
	health := int(float64(config.Health) * multiplier.HealthMultiplier)
	damage := int(float64(config.Damage) * multiplier.DamageMultiplier)
	speed := config.Speed * multiplier.SpeedMultiplier

	// Базовая конфигурация радиусов для зон (можно настроить)
	baseRadius := config.Radius
	// Добавляем случайное отклонение ±5
	randomOffset := (rand.Float64() * 10) - 5
	radius := baseRadius*multiplier.RadiusMultiplier + randomOffset

	// Гарантируем минимальный радиус
	if radius < 2.0 {
		radius = 2.0
	}

	return health, damage, speed, radius
}

func NewMob(id string, mobType MobType, x, y float64, zone string) *Mob {
	// Определяем редкость для этой зоны
	rarity := getRandomRarity(zone)

	// Применяем множители редкости к базовым характеристикам
	health, damage, speed, radius := applyRarityMultipliers(mobType, rarity, zone)

	return &Mob{
		ID:             id,
		Type:           mobType,
		Rarity:         rarity,
		Health:         health,
		MaxHealth:      health,
		Damage:         damage,
		Speed:          speed,
		X:              x,
		Y:              y,
		Zone:           zone,
		Radius:         radius,
		DetectionRange: MobConfigs[mobType].DetectionRange,
		LastMoveTime:   time.Now(),
		CreationTime:   time.Now(),
		LastHitTime:    time.Now(),
		State:          MobStateWandering,
	}
}

func (m *Mob) DistanceTo(otherX, otherY float64) float64 {
	dx := m.X - otherX
	dy := m.Y - otherY
	return math.Sqrt(dx*dx + dy*dy)
}

func (m *Mob) SetRandomTarget() {
	angle := rand.Float64() * 2 * math.Pi
	distance := 50 + rand.Float64()*100

	newTargetX := m.X + math.Cos(angle)*distance
	newTargetY := m.Y + math.Sin(angle)*distance

	// Проверяем, что новая цель достаточно далеко
	if math.Abs(newTargetX-m.X) > 10 || math.Abs(newTargetY-m.Y) > 10 {
		m.TargetX = newTargetX
		m.TargetY = newTargetY
		m.LastMoveTime = time.Now()
	}
}

// TakeDamage наносит урон мобу
func (m *Mob) TakeDamage(damage int) {
	m.Health -= damage
	if m.Health < 0 {
		m.Health = 0
	}
}

// IsAlive проверяет, жив ли моб
func (m *Mob) IsAlive() bool {
	return m.Health > 0
}

// CanAttack проверяет, может ли моб атаковать (прошло ли 100мс с последней атаки)
func (m *Mob) CanAttack() bool {
	return time.Since(m.LastHitTime) >= 100*time.Millisecond
}

// MarkAttack отмечает время атаки
func (m *Mob) MarkAttack() {
	m.LastHitTime = time.Now()
}

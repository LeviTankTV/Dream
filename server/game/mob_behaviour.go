package game

import (
	"math"
	"math/rand"
	"time"
)

/*************  ✨ Windsurf Command ⭐  *************/
// UpdateMobs обновляет поведение всех мобов в игре,
// а затем обрабатывает коллизии между ними.
// Она вызывается в основном цикле игры.
//
/*******  306d4c9a-6a8a-482f-88f9-4bd2222882dd  *******/const (
	MobCollisionBuffer = 2.0  // Буфер между мобами
	MobAvoidanceForce  = 1.5  // Сила избегания других мобов
)


func (g *Game) UpdateMobs() {
    // Сначала собираем всех мобов под RLock
    g.mobsMu.RLock()
    mobs := make([]*Mob, 0, len(g.mobs))
    for _, mob := range g.mobs {
        mobs = append(mobs, mob)
    }
    g.mobsMu.RUnlock()
    
    // Обновляем поведение без блокировок
    for _, mob := range mobs {
        g.updateMobBehavior(mob)
    }
    
    // Разрешаем коллизии с минимальными блокировками
    g.resolveMobCollisions()
}

func (g *Game) resolveMobCollisions() {
	mobs := make([]*Mob, 0, len(g.mobs))
	for _, mob := range g.mobs {
		mobs = append(mobs, mob)
	}

	// Проверяем коллизии для каждой пары мобов
	for i := 0; i < len(mobs); i++ {
		for j := i + 1; j < len(mobs); j++ {
			mobA := mobs[i]
			mobB := mobs[j]

			// Проверяем, находятся ли мобы в одной зоне
			if mobA.Zone != mobB.Zone {
				continue
			}

			// Вычисляем расстояние между мобами
			dx := mobA.X - mobB.X
			dy := mobA.Y - mobB.Y
			distance := math.Sqrt(dx*dx + dy*dy)
			
			minDistance := mobA.Radius + mobB.Radius + MobCollisionBuffer

			if distance < minDistance && distance > 0 {
				// Мобы пересекаются - раздвигаем их
				overlap := minDistance - distance
				
				// Нормализуем вектор
				dx = dx / distance
				dy = dy / distance

				// Сдвигаем мобов в противоположных направлениях
				shift := overlap * 0.5 * MobAvoidanceForce

				mobA.X += dx * shift
				mobA.Y += dy * shift
				mobB.X -= dx * shift
				mobB.Y -= dy * shift

				// Также корректируем целевые позиции, чтобы мобы не пытались пройти друг через друга
				g.adjustMobTargets(mobA, mobB, dx, dy, shift)
			}
		}
	}
}

func (g *Game) adjustMobTargets(mobA, mobB *Mob, dx, dy, shift float64) {
	// Если мобы движутся друг на друга, корректируем их цели
	distanceToTargetA := math.Sqrt(math.Pow(mobA.TargetX-mobA.X, 2) + math.Pow(mobA.TargetY-mobA.Y, 2))
	distanceToTargetB := math.Sqrt(math.Pow(mobB.TargetX-mobB.X, 2) + math.Pow(mobB.TargetY-mobB.Y, 2))

	// Корректируем цели только если мобы активно движутся
	if distanceToTargetA > 10 {
		// Сдвигаем цель моба A в сторону от моба B
		angle := math.Atan2(dy, dx) + (math.Pi/2)*0.7
		avoidDistance := mobA.Radius + mobB.Radius + 20

		mobA.TargetX = mobA.X + math.Cos(angle)*avoidDistance
		mobA.TargetY = mobA.Y + math.Sin(angle)*avoidDistance
	}

	if distanceToTargetB > 10 {
		// Сдвигаем цель моба B в сторону от моба A
		angle := math.Atan2(dy, dx) - (math.Pi/2)*0.7
		avoidDistance := mobA.Radius + mobB.Radius + 20

		mobB.TargetX = mobB.X + math.Cos(angle)*avoidDistance
		mobB.TargetY = mobB.Y + math.Sin(angle)*avoidDistance
	}
}

func (g *Game) updateMobBehavior(mob *Mob) {
	now := time.Now()

	// Находим ближайшего игрока в зоне
	closestPlayer, distance := g.findClosestPlayerInZone(mob.X, mob.Y, mob.Zone)

	// Проверяем коллизии с другими мобами перед обновлением поведения
	g.avoidOtherMobs(mob)

	switch mob.Type {
	case MobTypeOrc:
		g.updateOrcBehavior(mob, closestPlayer, distance, now)
	case MobTypeWolf:
		g.updateWolfBehavior(mob, closestPlayer, distance, now)
	case MobTypeGoblin:
		g.updateGoblinBehavior(mob, closestPlayer, distance, now)
	}

	// Применяем движение
	g.moveMob(mob)
}

func (g *Game) avoidOtherMobs(mob *Mob) {
	// Избегаем столкновений с другими мобами при выборе пути
	for _, otherMob := range g.mobs {
		if otherMob.ID == mob.ID || otherMob.Zone != mob.Zone {
			continue
		}

		distance := mob.DistanceTo(otherMob.X, otherMob.Y)
		minDistance := mob.Radius + otherMob.Radius + MobCollisionBuffer + 10

		if distance < minDistance {
			// Вычисляем вектор избегания
			angle := math.Atan2(mob.Y-otherMob.Y, mob.X-otherMob.X)
			avoidDistance := minDistance + 30

			// Корректируем целевую позицию
			mob.TargetX = mob.X + math.Cos(angle)*avoidDistance
			mob.TargetY = mob.Y + math.Sin(angle)*avoidDistance
			mob.LastMoveTime = time.Now()
			break
		}
	}
}


func (g *Game) updateOrcBehavior(mob *Mob, player *Player, distance float64, now time.Time) {
	// Базовые скорости (настроить под ваш геймплей)
	const baseWanderSpeed = 0.8
	const baseChaseSpeed = 18

	if player != nil && distance <= mob.DetectionRange {
		if mob.State != MobStateChasing && mob.State != MobStateAttacking {
			mob.State = MobStateChasing
			mob.TargetPlayer = player.ID
		}

		// Вычисляем дистанцию атаки в переменной
		attackRange := mob.Radius + PlayerRadius + 10

		if distance <= attackRange {
			// Атака
			if now.After(mob.AttackCooldown) {
				mob.State = MobStateAttacking
				mob.AttackCooldown = now.Add(2 * time.Second)

				angle := math.Atan2(player.Y-mob.Y, player.X-mob.X)
				mob.TargetX = player.X - math.Cos(angle)*(mob.Radius+PlayerRadius+5)
				mob.TargetY = player.Y - math.Sin(angle)*(mob.Radius+PlayerRadius+5)

				// Сбрасываем скорость при атаке
				mob.Speed = baseChaseSpeed
			}
		} else {
			// Преследование с улучшенным зигзагом
			mob.State = MobStateChasing

			if now.Sub(mob.LastMoveTime) > 300*time.Millisecond {
				baseAngle := math.Atan2(player.Y-mob.Y, player.X-mob.X)

				// Время для плавных волн
				elapsed := now.Sub(mob.CreationTime).Seconds()

				// Многократные волны для сложного паттерна
				sinWave1 := math.Sin(elapsed*3) * 0.8
				sinWave2 := math.Sin(elapsed*1.5) * 1.2
				cosWave := math.Cos(elapsed*2) * 0.6

				// Комбинируем волны для сложного паттерна
				deviation := (sinWave1 + sinWave2 + cosWave) * 0.4

				// Добавляем случайный элемент для непредсказуемости
				randomFactor := (rand.Float64() - 0.5) * 0.3
				finalDeviation := deviation + randomFactor

				// Применяем отклонение
				finalAngle := baseAngle + finalDeviation

				// Дистанция до цели зависит от расстояния до игрока
				targetDistance := distance * 0.3
				if targetDistance > 100 {
					targetDistance = 100
				}
				if targetDistance < 40 {
					targetDistance = 40
				}

				mob.TargetX = player.X - math.Cos(finalAngle)*targetDistance
				mob.TargetY = player.Y - math.Sin(finalAngle)*targetDistance
				mob.LastMoveTime = now

				// Динамическая скорость для эффекта "завихрения"
				speedVariation := math.Abs(sinWave1) * 0.6
				mob.Speed = baseChaseSpeed + speedVariation
			}
		}
	} else {
		// Блуждание
		mob.State = MobStateWandering
		mob.TargetPlayer = ""
		mob.Speed = baseWanderSpeed // Меньшая скорость при блуждании

		if now.Sub(mob.LastMoveTime) > 3*time.Second {
			mob.SetRandomTarget()
		}
	}
}

func (g *Game) updateWolfBehavior(mob *Mob, player *Player, distance float64, now time.Time) {
	// Нейтральное поведение - просто бродит
	if mob.State != MobStateWandering || now.Sub(mob.LastMoveTime) > 3*time.Second {
		mob.State = MobStateWandering
		mob.SetRandomTarget()
	}
}

func (g *Game) updateGoblinBehavior(mob *Mob, player *Player, distance float64, now time.Time) {
	if player != nil && distance <= mob.DetectionRange {
		mob.State = MobStateFleeing
		// Убегает от игрока
		angle := math.Atan2(mob.Y-player.Y, mob.X-player.X)
		fleeDistance := 200.0

		mob.TargetX = mob.X + math.Cos(angle)*fleeDistance
		mob.TargetY = mob.Y + math.Sin(angle)*fleeDistance
		mob.LastMoveTime = now
	} else {
		mob.State = MobStateWandering
		if now.Sub(mob.LastMoveTime) > 3*time.Second {
			mob.SetRandomTarget()
		}
	}
}

func (g *Game) moveMob(mob *Mob) {
	if mob.TargetX == 0 && mob.TargetY == 0 {
		return
	}

	// Плавное движение к цели
	dx := mob.TargetX - mob.X
	dy := mob.TargetY - mob.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	if distance > 5 { // Минимальная дистанция до цели
		// Нормализация
		dx = dx / distance
		dy = dy / distance

		// Движение с учетом скорости
		moveX := dx * mob.Speed
		moveY := dy * mob.Speed

		newX := mob.X + moveX
		newY := mob.Y + moveY

		// Применяем ограничения зоны
		newX, newY = g.constrainMobToZone(mob, newX, newY)

		mob.X = newX
		mob.Y = newY
	}
}

func (g *Game) findClosestPlayerInZone(x, y float64, zone string) (*Player, float64) {
	g.playersMu.RLock()
	defer g.playersMu.RUnlock()

	var closestPlayer *Player
	minDistance := math.MaxFloat64

	for _, player := range g.players {
		if player.CurrentZone == zone {
			distance := math.Sqrt(math.Pow(x-player.X, 2) + math.Pow(y-player.Y, 2))
			if distance < minDistance {
				minDistance = distance
				closestPlayer = player
			}
		}
	}

	return closestPlayer, minDistance
}

func (g *Game) constrainMobToZone(mob *Mob, newX, newY float64) (float64, float64) {
	zone := g.zones[mob.Zone]
	if zone == nil {
		return newX, newY
	}

	if newX < zone.MinX {
		newX = zone.MinX
	} else if newX > zone.MaxX {
		newX = zone.MaxX
	}

	if newY < zone.MinY {
		newY = zone.MinY
	} else if newY > zone.MaxY {
		newY = zone.MaxY
	}

	return newX, newY
}

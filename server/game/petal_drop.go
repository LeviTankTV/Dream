package game

import "time"

type PetalDrop struct {
	ID       string    `json:"id"`
	Type     PetalType `json:"type"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	OwnerID  string    `json:"owner_id"`
	Zone     string    `json:"zone"`
	Created  time.Time `json:"-"`
	Lifetime time.Duration `json:"-"`
}

func (d *PetalDrop) IsExpired() bool {
	return time.Since(d.Created) > d.Lifetime
}

func (d *PetalDrop) CanBePickedBy(playerID string) bool {
	return d.OwnerID == playerID
}
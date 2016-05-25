package common

import (
	"fmt"
	"time"
)

type Dimension struct {
	Timestamp time.Time
	Value     int
}

func (d Dimension) String() string {
	return fmt.Sprintf("%s %d", d.Timestamp.Format(time.UnixDate), d.Value)
}

func NewDimension() *Dimension {
	return &Dimension{}
}

func (s Dimension) Eq(d Dimension) bool {
	if s.Timestamp == d.Timestamp && s.Value == d.Value {
		return true
	}
	return false
}

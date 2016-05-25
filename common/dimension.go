package common

import (
	"time"
	"fmt"
)

type Dimension struct {
	Timestamp time.Time
	Value	int
}

func (d Dimension) String() string {
	return fmt.Sprintf("%s %d", d.Timestamp.Format(time.UnixDate), d.Value)
}

func NewDimension() *Dimension {
	return &Dimension{}
}
